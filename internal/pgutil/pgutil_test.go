package pgutil

import (
	"testing"
)

func TestComputeLag(t *testing.T) {
	tests := []struct {
		name     string
		master   string
		slave    string
		segBytes int64
		wantLag  int64
		wantErr  bool
	}{
		{
			name:     "zero lag when positions are equal",
			master:   "B0/B4031000",
			slave:    "B0/B4031000",
			segBytes: 16777216,
			wantLag:  0,
			wantErr:  false,
		},
		{
			name:     "positive lag when master is ahead by offset",
			master:   "0/2000",
			slave:    "0/1000",
			segBytes: 16777216,
			wantLag:  4096, // 0x2000 - 0x1000 = 0x1000 = 4096
			wantErr:  false,
		},
		{
			name:     "positive lag across segments",
			master:   "2/0",
			slave:    "1/0",
			segBytes: 16777216,
			wantLag:  16777216, // 1 segment difference
			wantErr:  false,
		},
		{
			name:     "lag with both segment and offset difference",
			master:   "3/1000",
			slave:    "1/0",
			segBytes: 16777216,
			wantLag:  2*16777216 + 4096, // 2 segments + 0x1000 offset
			wantErr:  false,
		},
		{
			name:     "negative lag when slave is ahead",
			master:   "0/1000",
			slave:    "0/2000",
			segBytes: 16777216,
			wantLag:  -4096,
			wantErr:  false,
		},
		{
			name:     "invalid master format missing slash",
			master:   "B0B4031000",
			slave:    "B0/B4031000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "invalid slave format missing slash",
			master:   "B0/B4031000",
			slave:    "B0B4031000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "empty master",
			master:   "",
			slave:    "B0/B4031000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "invalid hex in master segment",
			master:   "ZZ/1000",
			slave:    "0/1000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "invalid hex in master offset",
			master:   "0/ZZZZ",
			slave:    "0/1000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "invalid hex in slave segment",
			master:   "0/1000",
			slave:    "GG/1000",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "invalid hex in slave offset",
			master:   "0/1000",
			slave:    "0/XXXX",
			segBytes: 16777216,
			wantErr:  true,
		},
		{
			name:     "realistic WAL positions",
			master:   "B0/B4031000",
			slave:    "B0/B4030000",
			segBytes: 16777216,
			wantLag:  4096, // 0xB4031000 - 0xB4030000 = 0x1000 = 4096
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lag, err := ComputeLag(tt.master, tt.slave, tt.segBytes)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ComputeLag() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ComputeLag() unexpected error: %v", err)
				return
			}
			if lag != tt.wantLag {
				t.Errorf("ComputeLag() = %d, want %d", lag, tt.wantLag)
			}
		})
	}
}
