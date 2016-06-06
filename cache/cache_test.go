package cache

import (
	"bytes"
	"testing"
	"time"
)

type testJunkType struct {
	someInt    int
	someString string
	someBytes  []byte
}

func TestCache(t *testing.T) {
	c := NewCache()

	v, err := c.Read("nothing-here")
	if err != nil {
		t.Fatal("got an error while reading non-existent key:", err)
	}
	if v != nil {
		t.Fatal("the value of a non-existen key was not nil:", v)
	}

	c.Write("someint", 5)
	c.Write("somestring", "a string")
	c.Write("sometype", testJunkType{1, "hello", []byte("world")})
	// We will be checking this towards the end of the test, so hopefully a
	// nanosecond is enough to expire this entry. Because go doesn't have
	// Time::Fake.
	c.WriteTimeout("expiring", 50, time.Nanosecond)
	// Note that we are setting the expiration and then writing a new (different
	// type!) value to the same key. So alsoexpiring should not have an
	// expiration and should actually be a string.
	c.WriteTimeout("alsoexpiring", 66, time.Nanosecond)
	c.Write("alsoexpiring", "the cake is a lie")

	v, err = c.Read("someint")
	if err != nil {
		t.Fatal("got error while reading someint:", err)
	}
	switch v.(type) {
	case int:
	default:
		t.Fatalf("the type of v returned for someint was not int: %T\n", v)
	}
	i := v.(int)
	if i != 5 {
		t.Fatal("got the wrong value for someint (expected 5):", i)
	}

	v, err = c.Read("somestring")
	if err != nil {
		t.Fatal("got error while reading somestinrg:", err)
	}
	switch v.(type) {
	case string:
	default:
		t.Fatalf("the type of v returned for somestring was not string: %T\n", v)
	}
	s := v.(string)
	if s != "a string" {
		t.Fatal("got the wrong value for somestring (expected \"a string\"):", s)
	}

	v, err = c.Read("sometype")
	if err != nil {
		t.Fatal("got error while reading sometype:", err)
	}
	switch v.(type) {
	case testJunkType:
	default:
		t.Fatalf("the type of v returned for sometype was not testJunkType: %T\n", v)
	}
	x := v.(testJunkType)
	if x.someInt != 1 || x.someString != "hello" || bytes.Compare(x.someBytes, []byte("world")) != 0 {
		t.Fatalf("the got the wrong value for sometype (expected 1, \"hello\", 'world'): %+v", x)
	}

	s, err = c.ReadString("somestring")
	if err != nil {
		t.Fatal("got error while reading somestring as a string:", err)
	}
	if s != "a string" {
		t.Fatal("got the wrong string for somestring as string (expected \"a string\"):", s)
	}

	s, err = c.ReadString("someint")
	if err != ErrWrongType {
		t.Fatal("got the wrong error while trying to read someint as a string:", err)
	}
	if s != "" {
		t.Fatal("expected an empty string when reading someint as a string:", s)
	}

	v, err = c.Read("expiring")
	if err != ErrTimeoutExpired {
		t.Fatal("the expiring value did not seem to expire:", err)
	}
	i = v.(int)
	if i != 50 {
		t.Fatal("did not get the old value we expected:", i)
	}

	s, err = c.ReadString("expiring")
	if err != ErrWrongType {
		t.Fatal("expected the wrong type error to trump the expiration error:", err)
	}

	s, err = c.ReadString("alsoexpiring")
	if err != nil {
		t.Fatal("setting a new value should not care about the old one or the old timeout:", err)
	}
	if s != "the cake is a lie" {
		t.Fatal("got the wrong string for alsoexpiring:", s)
	}

	c.Clear("expiring")
	s, err = c.ReadString("expiring")
	if err != nil {
		t.Fatal("there shouldn't be a problem reading a string that no longer exist:", err)
	}
	if s != "" {
		t.Fatal("expected the string returned for a deleted key to be empty:", s)
	}
}
