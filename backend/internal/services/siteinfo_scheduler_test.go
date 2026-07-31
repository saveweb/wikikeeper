package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestGroupUsesRegistrableDomain(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "Fandom subdomain", url: "https://ifmovie.fandom.com", want: "fandom.com"},
		{name: "another Fandom subdomain", url: "https://starwars.fandom.com/wiki/Main_Page", want: "fandom.com"},
		{name: "multi-part public suffix", url: "https://wiki.example.co.uk", want: "example.co.uk"},
		{name: "IP address", url: "http://127.0.0.1:8080", want: "127.0.0.1"},
		{name: "invalid URL", url: "://invalid", want: "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, requestGroup(tt.url))
		})
	}
}
