package password

import "testing"

func TestGeneratePassword(t *testing.T) {
	type args struct {
		password string
	}
	tests := []struct {
		name     string
		args     args
		notEmpty bool
	}{
		{
			"Generate password",
			args{
				password: "password",
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GeneratePassword(tt.args.password); (got != "") != tt.notEmpty {
				t.Errorf("GeneratePassword() = %v, want %v", (got != ""), tt.notEmpty)
			}
		})
	}
}

func TestComparePasswords(t *testing.T) {
	type args struct {
		hashedPwd string
		inputPwd  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"match password",
			args{
				hashedPwd: "$2a$04$.fPfNsC89pIKHdzRb5pt1.75X4i7HD4EzcWQGv2/i90M8Wz6ya/Zy",
				inputPwd:  "password",
			},
			true,
		},
		{
			"wrong password",
			args{
				hashedPwd: "$2a$04$.fPfNsC89pIKHdzRb5pt1.75X4i7HD4EzcWQGv2/i90M8Wz6ya/Zy",
				inputPwd:  "passwordWrong",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComparePasswords(tt.args.hashedPwd, tt.args.inputPwd); got != tt.want {
				t.Errorf("ComparePasswords() = %v, want %v", got, tt.want)
			}
		})
	}
}
