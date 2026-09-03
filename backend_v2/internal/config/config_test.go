package config

import "testing"

func TestAlpacaStreamRequiresExplicitEnablement(t *testing.T) {
	t.Setenv("ALPACA_API_KEY", "test-key")
	t.Setenv("ALPACA_STREAM_ENABLED", "")
	if cfg := LoadAlpaca(); cfg.StreamConfigured() {
		t.Fatal("stream enabled without ALPACA_STREAM_ENABLED")
	}

	t.Setenv("ALPACA_STREAM_ENABLED", "true")
	if cfg := LoadAlpaca(); !cfg.StreamConfigured() {
		t.Fatal("configured stream was not enabled")
	}
}

func TestAlpacaStreamStillRequiresCredentials(t *testing.T) {
	t.Setenv("ALPACA_API_KEY", "")
	t.Setenv("ALPACA_STREAM_ENABLED", "true")
	if cfg := LoadAlpaca(); cfg.StreamConfigured() {
		t.Fatal("stream enabled without credentials")
	}
}
