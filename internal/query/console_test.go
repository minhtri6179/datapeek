package query

import (
	"testing"

	"datapeek/internal/config"
)

func TestClassifyReadOnly(t *testing.T) {
	cases := []struct {
		sql   string
		verb  string
		ro    bool
		db    config.DBType
	}{
		{"select * from t", "select", true, config.MySQL},
		{"SELECT 1", "select", true, config.MySQL},
		{"  select 1;  ", "select", true, config.MySQL},
		{"show tables", "show", true, config.MySQL},
		{"explain select * from t", "explain", true, config.MySQL},
		{"describe users", "describe", true, config.MySQL},
		{"desc users", "desc", true, config.MySQL},
		{"with x as (select 1) select * from x", "with", true, config.PostgreSQL},
	}
	for _, c := range cases {
		verb, ro, err := ClassifyStatement(c.db, c.sql)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", c.sql, err)
		}
		if verb != c.verb {
			t.Fatalf("%q: verb = %q want %q", c.sql, verb, c.verb)
		}
		if !ro != !c.ro {
			t.Fatalf("%q: expected read-only=%v, got %v", c.sql, c.ro, ro)
		}
	}
}

func TestClassifyWriteVerbs(t *testing.T) {
	cases := []string{
		"insert into t values (1)",
		"update t set a = 1",
		"delete from t",
		"truncate table t",
		"create table x (id int)",
		"drop table x",
		"alter table x add column y int",
		"set @a = 1",
		"lock tables t write",
		"grant all on *.* to u",
		"call do_stuff()",
	}
	for _, sql := range cases {
		verb, ro, err := ClassifyStatement(config.MySQL, sql)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", sql, err)
		}
		if ro {
			t.Fatalf("%q: expected write (verb %q)", sql, verb)
		}
	}
}

func TestClassifyHiddenWrites(t *testing.T) {
	cases := []struct {
		sql   string
		db    config.DBType
		reason string
	}{
		{"with x as (delete from t returning *) select * from x", config.PostgreSQL, "data-modifying CTE"},
		{"explain update t set a = 1", config.MySQL, "explain analyze executes DML"},
		{"explain analyze delete from t", config.PostgreSQL, "explain analyze executes DML"},
		{"select * into new_t from t", config.PostgreSQL, "select into creates table"},
		{"select * from t into outfile '/tmp/x'", config.MySQL, "outfile"},
		{"select * from t into dumpfile '/tmp/x'", config.MySQL, "dumpfile"},
	}
	for _, c := range cases {
		_, ro, err := ClassifyStatement(c.db, c.sql)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", c.sql, err)
		}
		if ro {
			t.Fatalf("%q: expected write classification (%s)", c.sql, c.reason)
		}
	}
}

func TestClassifyMultiStatement(t *testing.T) {
	cases := []string{
		"select 1; select 2",
		"select 1; drop table t",
		"update t set a=1; select 1",
	}
	for _, sql := range cases {
		if _, _, err := ClassifyStatement(config.MySQL, sql); err == nil {
			t.Fatalf("%q: expected multi-statement error", sql)
		}
	}
}

func TestClassifySemicolonsInsideLiterals(t *testing.T) {
	_, ro, err := ClassifyStatement(config.MySQL, "select ';' from t")
	if err != nil {
		t.Fatal(err)
	}
	if !ro {
		t.Fatal("expected read-only")
	}
	_, _, err = ClassifyStatement(config.MySQL, "delete from t where note = 'a;b'")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClassifyComments(t *testing.T) {
	cases := []string{
		"-- leading comment\nselect 1",
		"/* block\ncomment */ select 1",
		"select 1 -- trailing comment with ; semicolon",
		"select 1 /* inline ; comment */",
	}
	for _, sql := range cases {
		_, ro, err := ClassifyStatement(config.MySQL, sql)
		if err != nil {
			t.Fatalf("%q: %v", sql, err)
		}
		if !ro {
			t.Fatalf("%q: expected read-only", sql)
		}
	}
}

func TestClassifyErrors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		";;;",
		"select 'unterminated",
		"select `unterminated",
		"/* unterminated",
	}
	for _, sql := range cases {
		if _, _, err := ClassifyStatement(config.MySQL, sql); err == nil {
			t.Fatalf("%q: expected error", sql)
		}
	}
}

func TestClassifyQuotedIdentifierKeywords(t *testing.T) {
	_, ro, err := ClassifyStatement(config.PostgreSQL, `with cte as (select "update" from t) select * from cte`)
	if err != nil {
		t.Fatal(err)
	}
	if !ro {
		t.Fatal("quoted column name should not disable read-only")
	}
}

func TestMySQLSelectIntoVar(t *testing.T) {
	_, ro, err := ClassifyStatement(config.MySQL, "select count(*) into @n from t")
	if err != nil {
		t.Fatal(err)
	}
	if !ro {
		t.Fatal("mysql select into variable should be read-only")
	}
}