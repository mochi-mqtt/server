// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package auth

import (
	"encoding/json"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	Deny      Access = iota // user cannot access the topic
	ReadOnly                // user can only subscribe to the topic
	WriteOnly               // user can only publish to the topic
	ReadWrite               // user can both publish and subscribe to the topic
)

// Access determines the read/write privileges for an ACL rule.
type Access byte

// Users contains a map of access rules for specific users, keyed on username.
type Users map[string]UserRule

// UserRule defines a set of access rules for a specific user.
type UserRule struct {
	Username RString `json:"username,omitempty" yaml:"username,omitempty"` // the username of a user
	Password RString `json:"password,omitempty" yaml:"password,omitempty"` // the password of a user
	ACL      Filters `json:"acl,omitempty" yaml:"acl,omitempty"`           // filters to match, if desired
	Disallow bool    `json:"disallow,omitempty" yaml:"disallow,omitempty"` // allow or disallow the user
}

// AuthRules defines generic access rules applicable to all users.
type AuthRules []AuthRule

type AuthRule struct {
	Client   RString `json:"client,omitempty" yaml:"client,omitempty"`     // the id of a connecting client
	Username RString `json:"username,omitempty" yaml:"username,omitempty"` // the username of a user
	Remote   RString `json:"remote,omitempty" yaml:"remote,omitempty"`     // remote address or
	Password RString `json:"password,omitempty" yaml:"password,omitempty"` // the password of a user
	Allow    bool    `json:"allow,omitempty" yaml:"allow,omitempty"`       // allow or disallow the users
}

// ACLRules defines generic topic or filter access rules applicable to all users.
type ACLRules []ACLRule

// ACLRule defines access rules for a specific topic or filter.
type ACLRule struct {
	Client   RString `json:"client,omitempty" yaml:"client,omitempty"`     // the id of a connecting client
	Username RString `json:"username,omitempty" yaml:"username,omitempty"` // the username of a user
	Remote   RString `json:"remote,omitempty" yaml:"remote,omitempty"`     // remote address or
	Filters  Filters `json:"filters,omitempty" yaml:"filters,omitempty"`   // filters to match
}

// Filters is a map of Access rules keyed on filter.
type Filters map[RString]Access

// RString is a rule value string.
type RString string

// Matches returns true if the rule matches a given string.
func (r RString) Matches(a string) bool {
	rr := string(r)
	if r == "" || r == "*" || a == rr {
		return true
	}

	i := strings.Index(rr, "*")
	if i > 0 && len(a) > i && strings.Compare(rr[:i], a[:i]) == 0 {
		return true
	}

	return false
}

// FilterMatches returns true if a filter matches a topic rule.
func (r RString) FilterMatches(a string) bool {
	_, ok := MatchTopic(string(r), a)
	return ok
}

// MatchTopic checks if a given topic matches a filter, accounting for filter
// wildcards. Eg. filter /a/b/+/c == topic a/b/d/c.
func MatchTopic(filter string, topic string) (elements []string, matched bool) {
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	elements = make([]string, 0)
	for i := 0; i < len(filterParts); i++ {
		if i >= len(topicParts) {
			matched = false
			return
		}

		if filterParts[i] == "+" {
			elements = append(elements, topicParts[i])
			continue
		}

		if filterParts[i] == "#" {
			matched = true
			elements = append(elements, strings.Join(topicParts[i:], "/"))
			return
		}

		if filterParts[i] != topicParts[i] {
			matched = false
			return
		}
	}

	return elements, true
}

// Ledger is an auth ledger containing access rules for users and topics.
type Ledger struct {
	sync.Mutex `json:"-" yaml:"-"`
	Users      Users     `json:"users" yaml:"users"`
	Auth       AuthRules `json:"auth" yaml:"auth"`
	ACL        ACLRules  `json:"acl" yaml:"acl"`
}

// Update updates the internal values of the ledger.
func (l *Ledger) Update(ln *Ledger) {
	l.Lock()
	defer l.Unlock()
	l.Auth = ln.Auth
	l.ACL = ln.ACL
}

// AuthOk returns true if the rules indicate the user is allowed to authenticate.
func (l *Ledger) AuthOk(cl *mqtt.Client, pk packets.Packet) (n int, ok bool) {
	// If the users map is set, always check for a predefined user first instead
	// of iterating through global rules.
	if l.Users != nil {
		if u, ok := l.Users[string(cl.Properties.Username)]; ok &&
			u.Password != "" &&
			u.Password == RString(pk.Connect.Password) {
			return 0, !u.Disallow
		}
	}

	// If there's no users map, or no user was found, attempt to find a matching
	// rule (which may also contain a user).
	for n, rule := range l.Auth {
		if rule.Client.Matches(cl.ID) &&
			rule.Username.Matches(string(cl.Properties.Username)) &&
			rule.Password.Matches(string(pk.Connect.Password)) &&
			rule.Remote.Matches(cl.Net.Remote) {
			return n, rule.Allow
		}
	}

	return 0, false
}

// ACLOk returns true if the rules indicate the user is allowed to read or write to
// a specific filter or topic respectively, based on the `write` bool.
func (l *Ledger) ACLOk(cl *mqtt.Client, topic string, write bool) (n int, ok bool) {
	// If the users map is set, always check for a predefined user first instead
	// of iterating through global rules.
	if l.Users != nil {
		if u, ok := l.Users[string(cl.Properties.Username)]; ok && len(u.ACL) > 0 {
			for filter, access := range u.ACL {
				if filter.FilterMatches(topic) {
					if !write && (access == ReadOnly || access == ReadWrite) {
						return n, true
					} else if write && (access == WriteOnly || access == ReadWrite) {
						return n, true
					} else {
						return n, false
					}
				}
			}
		}
	}

	for n, rule := range l.ACL {
		if rule.Client.Matches(cl.ID) &&
			rule.Username.Matches(string(cl.Properties.Username)) &&
			rule.Remote.Matches(cl.Net.Remote) {
			if len(rule.Filters) == 0 {
				return n, true
			}

			if write {
				for filter, access := range rule.Filters {
					if access == WriteOnly || access == ReadWrite {
						if filter.FilterMatches(topic) {
							return n, true
						}
					}
				}
			}

			if !write {
				for filter, access := range rule.Filters {
					if access == ReadOnly || access == ReadWrite {
						if filter.FilterMatches(topic) {
							return n, true
						}
					}
				}
			}

			for filter := range rule.Filters {
				if filter.FilterMatches(topic) {
					return n, false
				}
			}
		}
	}

	return 0, true
}

// ToJSON encodes the values into a JSON string.
func (l *Ledger) ToJSON() (data []byte, err error) {
	return json.Marshal(l)
}

// ToYAML encodes the values into a YAML string.
func (l *Ledger) ToYAML() (data []byte, err error) {
	return yaml.Marshal(l)
}

// Unmarshal decodes a JSON or YAML string (such as a rule config from a file) into a struct.
func (l *Ledger) Unmarshal(data []byte) error {
	l.Lock()
	defer l.Unlock()
	if len(data) == 0 {
		return nil
	}

	if data[0] == '{' {
		return json.Unmarshal(data, l)
	}

	return yaml.Unmarshal(data, &l)
}
