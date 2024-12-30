package convert

import "testing"

type InStruct struct {
	ID string `json:"id"`
}

var formData map[string]string

func TestObjectToObject(t *testing.T) {
	type args struct {
		in  interface{}
		out interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{"Success",
			args{
				in:  InStruct{ID: "1"},
				out: "{ID:1}",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ObjectToObject(tt.args.in, tt.args.out)
		})
	}
}

func TestObjectToString(t *testing.T) {
	type args struct {
		data interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Success 1",
			args{
				data: InStruct{ID: "1"},
			},
			"{\"id\":\"1\"}",
		},
		{"Success 2",
			args{
				data: InStruct{ID: "2"},
			},
			"{\"id\":\"2\"}",
		},
		{"Failed",
			args{
				data: make(chan int),
			},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ObjectToString(tt.args.data); got != tt.want {
				t.Errorf("ObjectToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToObject(t *testing.T) {
	type args struct {
		in  string
		out interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{"Success 1",
			args{
				in:  "{\"id\":\"1\"}",
				out: InStruct{ID: "1"},
			},
		},
		{"Success 2",
			args{
				in:  "{\"id\":\"2\"}",
				out: InStruct{ID: "2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			StringToObject(tt.args.in, tt.args.out)
		})
	}
}

func TestConvertToWord(t *testing.T) {
	type args struct {
		number int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"minus 1234", args{number: -1234}, "minus seribu dua ratus tiga puluh empat"},
		{"nol", args{number: 0}, "nol"},
		{"satu", args{number: 1}, "satu"},
		{"10", args{number: 10}, "sepuluh"},
		{"11", args{number: 11}, "sebelas"},
		{"15", args{number: 15}, "lima belas"},
		{"101", args{number: 101}, "seratus satu"},
		{"110", args{number: 110}, "seratus sepuluh"},
		{"156", args{number: 156}, "seratus lima puluh enam"},
		{"1000", args{number: 1000}, "seribu"},
		{"2001", args{number: 2001}, "dua ribu satu"},
		{"1234", args{number: 1234}, "seribu dua ratus tiga puluh empat"},
		{"12134", args{number: 12134}, "dua belas ribu seratus tiga puluh empat"},
		{"201000", args{number: 201000}, "dua ratus satu ribu"},
		{"1501", args{number: 1500}, "seribu lima ratus"},
		{"1000000", args{number: 1000000}, "satu juta"},
		{"1000000000", args{number: 1000000000}, "satu miliyar"},
		{"1000000000000", args{number: 1000000000000}, "satu triliun"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertToWord(tt.args.number); got != tt.want {
				t.Errorf("ConvertToWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertEng(t *testing.T) {
	type args struct {
		number int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"-1",
			args{
				number: -1,
			},
			"minus one",
		},
		{
			"0",
			args{
				number: 0,
			},
			"zero",
		},
		{
			"1",
			args{
				number: 1,
			},
			"one",
		},
		{
			"12",
			args{
				number: 12,
			},
			"twelve",
		},
		{
			"35",
			args{
				number: 35,
			},
			"thirty-five",
		},
		{
			"135",
			args{
				number: 135,
			},
			"one hundred thirty-five",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertEng(tt.args.number); got != tt.want {
				t.Errorf("ConvertEng() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertEngAnd(t *testing.T) {
	type args struct {
		number int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"135",
			args{
				number: 135,
			},
			"one hundred and thirty-five",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertEngAnd(tt.args.number); got != tt.want {
				t.Errorf("ConvertEngAnd() = %v, want %v", got, tt.want)
			}
		})
	}
}
