package main

import "testing"

func TestParsePlayFlags(t *testing.T) {
	clean, flags := parsePlayFlags([]string{"-shuffle", "song", "--loop", "-priority", "another"})

	if len(clean) != 2 || clean[0] != "song" || clean[1] != "another" {
		t.Fatalf("unexpected clean args: %v", clean)
	}

	if !flags.random || !flags.loop || !flags.priority {
		t.Fatalf("expected all flags set, got %+v", flags)
	}
}

func TestParsePlayFlagsNone(t *testing.T) {
	clean, flags := parsePlayFlags([]string{"just", "a", "song"})

	if len(clean) != 3 {
		t.Fatalf("unexpected clean args: %v", clean)
	}

	if flags.random || flags.loop || flags.priority {
		t.Fatalf("expected no flags, got %+v", flags)
	}
}
