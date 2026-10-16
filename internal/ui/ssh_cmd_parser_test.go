package ui

import (
	"testing"
)

func TestParseSSHCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantOK       bool
		wantHost     string
		wantUser     string
		wantPort     int
		wantPassword string
	}{
		{
			name:   "not ssh",
			input:  "ls -la",
			wantOK: false,
		},
		{
			name:   "command with ssh in middle",
			input:  "echo ssh user@host",
			wantOK: false,
		},
		{
			name:     "bare ssh",
			input:    "ssh",
			wantOK:   true,
			wantHost: "",
			wantUser: "root",
			wantPort: 22,
		},
		{
			name:     "ssh host only",
			input:    "ssh 192.168.1.100",
			wantOK:   true,
			wantHost: "192.168.1.100",
			wantUser: "root",
			wantPort: 22,
		},
		{
			name:     "ssh user@host",
			input:    "ssh ubuntu@10.0.0.1",
			wantOK:   true,
			wantHost: "10.0.0.1",
			wantUser: "ubuntu",
			wantPort: 22,
		},
		{
			name:     "ssh user@host -p 2222",
			input:    "ssh admin@example.com -p 2222",
			wantOK:   true,
			wantHost: "example.com",
			wantUser: "admin",
			wantPort: 2222,
		},
		{
			name:     "ssh -p 2200 user@host",
			input:    "ssh -p 2200 root@myhost",
			wantOK:   true,
			wantHost: "myhost",
			wantUser: "root",
			wantPort: 2200,
		},
		{
			name:     "ssh -p2200 user@host",
			input:    "ssh -p2200 user@myhost",
			wantOK:   true,
			wantHost: "myhost",
			wantUser: "user",
			wantPort: 2200,
		},
		{
			name:     "ssh -l debian host",
			input:    "ssh -l debian 172.16.0.5",
			wantOK:   true,
			wantHost: "172.16.0.5",
			wantUser: "debian",
			wantPort: 22,
		},
		{
			name:     "ssh user@host:8022",
			input:    "ssh dev@192.168.2.1:8022",
			wantOK:   true,
			wantHost: "192.168.2.1",
			wantUser: "dev",
			wantPort: 8022,
		},
		{
			name:         "ssh user:pwd@host:2222",
			input:        "ssh root:secret123@1.1.1.1:2222",
			wantOK:       true,
			wantHost:     "1.1.1.1",
			wantUser:     "root",
			wantPassword: "secret123",
			wantPort:     2222,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, ok := ParseSSHCommand(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ParseSSHCommand(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if host.Address != tt.wantHost {
				t.Errorf("Address = %q, want %q", host.Address, tt.wantHost)
			}
			if host.Username != tt.wantUser {
				t.Errorf("Username = %q, want %q", host.Username, tt.wantUser)
			}
			if host.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", host.Port, tt.wantPort)
			}
			if tt.wantPassword != "" && host.Password != tt.wantPassword {
				t.Errorf("Password = %q, want %q", host.Password, tt.wantPassword)
			}
			if !host.IsEphemeral {
				t.Errorf("IsEphemeral = false, want true")
			}
		})
	}
}
