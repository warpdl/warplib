package warplib

import (
	"testing"
)

func TestHeaders_Update(t *testing.T) {
	type args struct {
		key   string
		value string
	}
	tests := []struct {
		name string
		h    *Headers
		args args
	}{
		{
			"new entry", &Headers{}, args{__USER_AGENT_KEY, DEF_USER_AGENT},
		},
		{
			"existing entry", &Headers{{__USER_AGENT_KEY, "TestUA/12.3"}}, args{__USER_AGENT_KEY, DEF_USER_AGENT},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.h.Update(tt.args.key, tt.args.value)
			i, ok := tt.h.Get(__USER_AGENT_KEY)
			if !ok || (*tt.h)[i].value != tt.args.value {
				t.Errorf("Headers.Update() did not update: %v", tt.h)
			}
		})
	}
}
