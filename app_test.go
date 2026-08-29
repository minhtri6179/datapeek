package main

import (
	"testing"

	"datapeek/internal/config"
)

func TestValidateConn(t *testing.T) {
	valid := config.Connection{Name: "local", Type: config.MySQL, Host: "127.0.0.1", Port: 3306, User: "root", Database: "app"}
	if err := validateConn(valid); err != nil {
		t.Fatalf("valid conn rejected: %v", err)
	}
	pg := valid
	pg.Type = config.PostgreSQL
	pg.Port = 5432
	if err := validateConn(pg); err != nil {
		t.Fatalf("valid pg conn rejected: %v", err)
	}
}

func TestValidateConnErrors(t *testing.T) {
	cases := []config.Connection{
		{},                                                    // everything missing
		{Name: "x", Type: config.MySQL},                       // no host
		{Name: "x", Type: config.MySQL, Host: "h"},            // no user
		{Name: "x", Type: config.MySQL, Host: "h", User: "u"}, // no database
		{Name: "x", Type: "redis", Host: "h", User: "u", Database: "d"},
	}
	for i, c := range cases {
		if err := validateConn(c); err == nil {
			t.Fatalf("case %d: expected error for %+v", i, c)
		}
	}
	// port range
	c := config.Connection{Name: "x", Type: config.MySQL, Host: "h", User: "u", Database: "d"}
	c.Port = 0
	if err := validateConn(c); err == nil {
		t.Fatal("port 0 should fail")
	}
	c.Port = 70000
	if err := validateConn(c); err == nil {
		t.Fatal("port 70000 should fail")
	}
}

func TestListConnectionsEmpty(t *testing.T) {
	app := newAppAt(t.TempDir())
	conns, err := app.ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected empty, got %d", len(conns))
	}
}

func TestLiveConnNotConnected(t *testing.T) {
	app := newAppAt(t.TempDir())
	if _, _, err := app.liveConn("nope"); err == nil {
		t.Fatal("expected error for unknown connection")
	}
}
