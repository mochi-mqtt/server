package file

import (
	"fmt"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
)

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		want    *mqtt.Server
		wantErr bool
	}{
		{
			name:    "Sucesfull configuration",
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Configure()
			if err != nil {
				fmt.Println(err)
			}

		})
	}
}
