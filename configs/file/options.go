package file

import mqtt "github.com/mochi-mqtt/server/v2"

// Options contains configurable options for the server.
type Options struct {
	// Capabilities defines the server features and behaviour. If you only wish to modify
	// several of these values, set them explicitly - e.g.
	// 	server.Options.Capabilities.MaximumClientWritesPending = 16 * 1024
	Capabilities *Capabilities `yaml:"capabilities"`

	// ClientNetWriteBufferSize specifies the size of the client *bufio.Writer write buffer.
	ClientNetWriteBufferSize *int `yaml:"client_net_write_buffer_size"`

	// ClientNetReadBufferSize specifies the size of the client *bufio.Reader read buffer.
	ClientNetReadBufferSize *int `yaml:"client_net_read_buffer_size"`

	// SysTopicResendInterval specifies the interval between $SYS topic updates in seconds.
	SysTopicResendInterval *int64 `yaml:"sys_topic_resend_interval"`

	// Enable Inline client to allow direct subscribing and publishing from the parent codebase,
	// with negligible performance difference (disabled by default to prevent confusion in statistics).
	InlineClient *bool `yaml:"inline_client"`
}

// Capabilities indicates the capabilities and features provided by the server.
type Capabilities struct {
	MaximumMessageExpiryInterval *int64           `yaml:"maximum_message_expiry_interval"`
	MaximumClientWritesPending   *int32           `yaml:"maximum_client_writes_pending"`
	MaximumSessionExpiryInterval *uint32          `yaml:"maximum_session_expiry_interval"`
	MaximumPacketSize            *uint32          `yaml:"maximum_packet_size"`
	ReceiveMaximum               *uint16          `yaml:"receive_maximum"`
	TopicAliasMaximum            *uint16          `yaml:"topic_alias_maximum"`
	SharedSubAvailable           *byte            `yaml:"shared_sub_available"`
	MinimumProtocolVersion       *byte            `yaml:"minimum_protocol_version"`
	Compatibilities              *Compatibilities `yaml:"compatibilities"`
	MaximumQos                   *byte            `yaml:"maximum_qos"`
	RetainAvailable              *byte            `yaml:"retain_available"`
	WildcardSubAvailable         *byte            `yaml:"wildcard_sub_available"`
	SubIDAvailable               *byte            `yaml:"sub_id_available"`
}

// Compatibilities provides flags for using compatibility modes.
type Compatibilities struct {
	ObscureNotAuthorized       *bool `yaml:"obscure_not_authorized"`         // return unspecified errors instead of not authorized
	PassiveClientDisconnect    *bool `yaml:"passive_client_disconnect"`      // don't disconnect the client forcefully after sending disconnect packet (paho - spec violation)
	AlwaysReturnResponseInfo   *bool `yaml:"always_return_response_info"`    // always return response info (useful for testing)
	RestoreSysInfoOnRestart    *bool `yaml:"restore_sys_info_on_restart"`    // restore system info from store as if server never stopped
	NoInheritedPropertiesOnAck *bool `yaml:"no_inherited_properties_on_ack"` // don't allow inherited user properties on ack (paho - spec violation)
}

func CompareOptions(opts Options) mqtt.Options {
	ropts := mqtt.Options{}
	ropts.EnsureDefaults()

	if opts.Capabilities != nil {
		if opts.Capabilities.Compatibilities != nil {
			if opts.Capabilities.Compatibilities.AlwaysReturnResponseInfo != nil {
				ropts.Capabilities.Compatibilities.AlwaysReturnResponseInfo = *opts.Capabilities.Compatibilities.AlwaysReturnResponseInfo
			}
			if opts.Capabilities.Compatibilities.NoInheritedPropertiesOnAck != nil {
				ropts.Capabilities.Compatibilities.NoInheritedPropertiesOnAck = *opts.Capabilities.Compatibilities.NoInheritedPropertiesOnAck
			}
			if opts.Capabilities.Compatibilities.ObscureNotAuthorized != nil {
				ropts.Capabilities.Compatibilities.ObscureNotAuthorized = *opts.Capabilities.Compatibilities.ObscureNotAuthorized
			}
			if opts.Capabilities.Compatibilities.PassiveClientDisconnect != nil {
				ropts.Capabilities.Compatibilities.PassiveClientDisconnect = *opts.Capabilities.Compatibilities.PassiveClientDisconnect
			}
			if opts.Capabilities.Compatibilities.RestoreSysInfoOnRestart != nil {
				ropts.Capabilities.Compatibilities.RestoreSysInfoOnRestart = *opts.Capabilities.Compatibilities.RestoreSysInfoOnRestart
			}
		}
		if opts.Capabilities.MaximumClientWritesPending != nil {
			ropts.Capabilities.MaximumClientWritesPending = *opts.Capabilities.MaximumClientWritesPending
		}
		if opts.Capabilities.MaximumMessageExpiryInterval != nil {
			ropts.Capabilities.MaximumMessageExpiryInterval = *opts.Capabilities.MaximumMessageExpiryInterval
		}
		if opts.Capabilities.MaximumPacketSize != nil {
			ropts.Capabilities.MaximumPacketSize = *opts.Capabilities.MaximumPacketSize
		}
		if opts.Capabilities.MaximumQos != nil {
			ropts.Capabilities.MaximumQos = *opts.Capabilities.MaximumQos
		}
		if opts.Capabilities.MaximumSessionExpiryInterval != nil {
			ropts.Capabilities.MaximumSessionExpiryInterval = *opts.Capabilities.MaximumSessionExpiryInterval
		}
		if opts.Capabilities.ReceiveMaximum != nil {
			ropts.Capabilities.ReceiveMaximum = *opts.Capabilities.ReceiveMaximum
		}
		if opts.Capabilities.RetainAvailable != nil {
			ropts.Capabilities.RetainAvailable = *opts.Capabilities.RetainAvailable
		}
		if opts.Capabilities.SharedSubAvailable != nil {
			ropts.Capabilities.SharedSubAvailable = *opts.Capabilities.SharedSubAvailable
		}
		if opts.Capabilities.TopicAliasMaximum != nil {
			ropts.Capabilities.TopicAliasMaximum = *opts.Capabilities.TopicAliasMaximum
		}
		if opts.Capabilities.WildcardSubAvailable != nil {
			ropts.Capabilities.WildcardSubAvailable = *opts.Capabilities.WildcardSubAvailable
		}
		if opts.Capabilities.MinimumProtocolVersion != nil {
			ropts.Capabilities.MinimumProtocolVersion = *opts.Capabilities.MinimumProtocolVersion
		}
	}
	if opts.ClientNetReadBufferSize != nil {
		ropts.ClientNetReadBufferSize = *opts.ClientNetReadBufferSize
	}
	if opts.ClientNetWriteBufferSize != nil {
		ropts.ClientNetWriteBufferSize = *opts.ClientNetWriteBufferSize
	}
	if opts.InlineClient != nil {
		ropts.InlineClient = *opts.InlineClient
	}
	if opts.SysTopicResendInterval != nil {
		ropts.SysTopicResendInterval = *opts.SysTopicResendInterval
	}

	return ropts
}
