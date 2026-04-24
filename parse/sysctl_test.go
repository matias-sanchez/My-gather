package parse

import "testing"

func TestParseSysctl_ExtractsOnlyKnownKeys(t *testing.T) {
	input := `kernel.osrelease = 4.18.0-553.89.1.el8_10.x86_64
kernel.version = #1 SMP Sat Nov 29 00:49:18 EST 2025
fs.file-max = 3253554
vm.swappiness = 30
vm.dirty_ratio = 30
vm.dirty_background_ratio = 10
crypto.fips_name = Red Hat Enterprise Linux 8 - Kernel Cryptographic API
net.core.rmem_max = 212992
`
	got := ParseSysctl(input)
	if got == nil {
		t.Fatalf("ParseSysctl returned nil")
	}
	if got["kernel.osrelease"] != "4.18.0-553.89.1.el8_10.x86_64" {
		t.Errorf("kernel.osrelease: got %q", got["kernel.osrelease"])
	}
	if got["crypto.fips_name"] != "Red Hat Enterprise Linux 8 - Kernel Cryptographic API" {
		t.Errorf("crypto.fips_name: got %q", got["crypto.fips_name"])
	}
	if got["vm.swappiness"] != "30" {
		t.Errorf("vm.swappiness: got %q", got["vm.swappiness"])
	}
	if _, ok := got["net.core.rmem_max"]; ok {
		t.Errorf("net.core.rmem_max should be filtered out")
	}
}

func TestParseSysctl_EmptyReturnsNil(t *testing.T) {
	if got := ParseSysctl(""); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}
