package fw

import "testing"

func Test_joinRoute(t *testing.T) {
	type args struct {
		base     string
		r        string
		override bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				base: "/api/",
				r:    "/",
			},
			want: "/api",
		},
		{
			name: "test2",
			args: args{
				base: "/api/",
				r:    "",
			},
			want: "/api",
		},
		{
			name: "test3",
			args: args{
				base: "/api",
				r:    "/",
			},
			want: "/api",
		},
		{
			name: "test4",
			args: args{
				base: "/api",
				r:    "",
			},
			want: "/api",
		},
		{
			name: "test5",
			args: args{
				base: "/api/",
				r:    "/one",
			},
			want: "/api/one",
		},
		{
			name: "test6",
			args: args{
				base: "/api/",
				r:    "one",
			},
			want: "/api/one",
		},
		{
			name: "test7",
			args: args{
				base: "/api",
				r:    "/one",
			},
			want: "/api/one",
		},
		{
			name: "test8",
			args: args{
				base: "/api",
				r:    "one",
			},
			want: "/api/one",
		},
		{
			name: "test9",
			args: args{
				base: "/",
				r:    "/",
			},
			want: "/",
		},
		{
			name: "test10",
			args: args{
				base: "",
				r:    "/",
			},
			want: "/",
		},
		{
			name: "test11",
			args: args{
				base:     "/api",
				r:        "/web",
				override: true,
			},
			want: "/web",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinRoute(tt.args.base, tt.args.r, tt.args.override); got != tt.want {
				t.Errorf("joinRoute() = %v, want %v", got, tt.want)
			}
		})
	}
}
