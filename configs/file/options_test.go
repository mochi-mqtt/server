package file

import (
	"reflect"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
)

func TestCheckForDefaults(t *testing.T) {
	var defaultOption mqtt.Options
	defaultOption.EnsureDefaults()

	type args struct {
		opts Options
	}
	tests := []struct {
		name string
		args args
		want mqtt.Options
	}{
		{
			name: "default options",
			args: args{
				opts: Options{},
			},
			want: defaultOption,
		},
		{
			name: "full options",
			args: args{
				opts: Options{
					Capabilities: &Capabilities{
						Compatibilities: &Compatibilities{
							AlwaysReturnResponseInfo:   boolPtr(true),
							NoInheritedPropertiesOnAck: boolPtr(true),
							ObscureNotAuthorized:       boolPtr(true),
							PassiveClientDisconnect:    boolPtr(true),
							RestoreSysInfoOnRestart:    boolPtr(true),
						},
						MaximumClientWritesPending:   int32Ptr(1),
						MaximumMessageExpiryInterval: int64Ptr(1),
						MaximumPacketSize:            uint32Ptr(1),
						MaximumQos:                   bytePtr(1),
						MaximumSessionExpiryInterval: uint32Ptr(1),
						ReceiveMaximum:               uint16Ptr(1),
						RetainAvailable:              bytePtr(1),
						SharedSubAvailable:           bytePtr(1),
						TopicAliasMaximum:            uint16Ptr(1),
						WildcardSubAvailable:         bytePtr(1),
						MinimumProtocolVersion:       bytePtr(1),
						SubIDAvailable:               bytePtr(1),
					},
					ClientNetReadBufferSize:  intPtr(1024 * 2),
					ClientNetWriteBufferSize: intPtr(1024 * 2),
					SysTopicResendInterval:   int64Ptr(1),
					InlineClient:             boolPtr(false),
				},
			},
			want: defaultOption,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckForDefaults(tt.args.opts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckForDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func bytePtr(b byte) *byte {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func uint16Ptr(i uint16) *uint16 {
	return &i
}

func uint32Ptr(i uint32) *uint32 {
	return &i
}
