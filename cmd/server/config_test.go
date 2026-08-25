package main

import "testing"

func TestAddressMustBeLoopback(t *testing.T) {
	if err := validateAddress("127.0.0.1:19081"); err != nil {
		t.Fatal(err)
	}
	if err := validateAddress("0.0.0.0:19081"); err == nil {
		t.Fatal("必须拒绝非回环监听")
	}
	if err := validateAddress("127.0.0.1:0"); err == nil {
		t.Fatal("必须拒绝零端口")
	}
}

func TestDefaultAddress(t *testing.T) {
	t.Setenv("PORT", "")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.address != defaultAddress {
		t.Fatalf("默认地址异常：%s", cfg.address)
	}
}
