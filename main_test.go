package main

import (
	"context"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"reflect"
	"testing"
)

func Test_vSphereDeployment_Increase(t *testing.T) {
	type fields struct {
		settings provider.Settings
	}
	type args struct {
		ctx context.Context
		n   int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "Case 1: Valid increase",
			fields: fields{
				settings: provider.Settings{
					// fill with your settings
				},
			},
			args: args{
				ctx: context.Background(), // for example, a background context
				n:   5,                    // example value for increasing
			},
			want:    5,     // expected result after increase
			wantErr: false, // we don't expect an error in this case
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &vSphereDeployment{
				settings: tt.fields.settings,
			}
			got, err := k.Increase(tt.args.ctx, tt.args.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("Increase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Increase() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vSphereDeployment_Decrease(t *testing.T) {
	type fields struct {
		settings provider.Settings
	}
	type args struct {
		ctx       context.Context
		instances []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "Case 1: Valid decrease",
			fields: fields{
				settings: provider.Settings{
					// fill with your settings
				},
			},
			args: args{
				ctx:       context.Background(),                                                                               // for example, a background context
				instances: []string{"ubuntu-child-1", "ubuntu-child-2", "ubuntu-child-3", "ubuntu-child-4", "ubuntu-child-5"}, // example instance names
			},
			want:    []string{"ubuntu-child-1", "ubuntu-child-2", "ubuntu-child-3", "ubuntu-child-4", "ubuntu-child-5"}, // expected result after decrease (for instance, "instance3" might have been removed)
			wantErr: false,                                                                                              // we don't expect an error in this case
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &vSphereDeployment{
				settings: tt.fields.settings,
			}
			got, err := k.Decrease(tt.args.ctx, tt.args.instances)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decrease() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vSphereDeployment_ConnectInfo(t *testing.T) {
	type fields struct {
		settings provider.Settings
	}
	type args struct {
		ctx      context.Context
		instance string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    provider.ConnectInfo
		wantErr bool
	}{
		{
			name: "Case 1: Valid IP",
			fields: fields{
				settings: provider.Settings{
					// fill with your settings
				},
			},
			args: args{
				ctx:      context.Background(), // for example, a background context
				instance: "vc-prod",            // example value for increasing
			},
			want: provider.ConnectInfo{
				ID:           "vc-prod",
				InternalAddr: "10.42.144.11",
			}, // expected result after increase
			wantErr: false, // we don't expect an error in this case
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &vSphereDeployment{
				settings: tt.fields.settings,
			}
			got, err := k.ConnectInfo(tt.args.ctx, tt.args.instance)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConnectInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vSphereDeployment_Update(t *testing.T) {
	type fields struct {
		settings provider.Settings
	}
	type args struct {
		ctx context.Context
		fn  func(instance string, state provider.State)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Case 1: Valid IP",
			fields: fields{
				settings: provider.Settings{
					// fill with your settings
				},
			},
			args: args{
				ctx: context.Background(), // for example, a background context
				fn:  echofunc,             // example value for increasing
			},
			wantErr: false, // we don't expect an error in this case
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &vSphereDeployment{
				settings: tt.fields.settings,
			}
			if err := k.Update(tt.args.ctx, tt.args.fn); (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func echofunc(instance string, state provider.State) {
	println(instance)
	println(state)
}
