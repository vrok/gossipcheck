package main

import (
	"gossipcheck/checks"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestLoadChecks(t *testing.T) {
	//checksFile = &"file.json"

	f, err := ioutil.TempFile("", "gcheck")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())

	err = ioutil.WriteFile(f.Name(), []byte(`
{
  "check_etc_hosts_has_8888": {
    "path": "/etc/hosts", 
    "type": "file_contains", 
    "check": "8.8.8.8"
  }, 
  "check_kite_config_file_exists": {
    "path": "/etc/host/koding/kite.conf", 
    "type": "file_exists"
  },
  "check_nginx_running": {
	"path": "/sbin/init",
	"action": "shutdown -r now"
  }
}
`), 0600)
	if err != nil {
		t.Error(err)
	}

	*checksFile = f.Name()
	p, err := loadChecks()
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(p, []*checks.Params{
		&checks.Params{Name: "check_etc_hosts_has_8888", Type: "file_contains", Path: "/etc/hosts", Check: "8.8.8.8"},
		&checks.Params{Name: "check_kite_config_file_exists", Type: "file_exists", Path: "/etc/host/koding/kite.conf"},
		&checks.Params{Name: "check_nginx_running", Path: "/sbin/init", Action: "shutdown -r now"},
	}) {
		t.Error("Error loading checks file")
	}
}
