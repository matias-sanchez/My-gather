package parse

import "testing"

func TestParseDFSnapshot_SortedByUsePctAndLimited(t *testing.T) {
	input := `TS 1769702259.004572779 2026-01-29 15:57:39
Filesystem                                           1K-blocks        Used   Available Use% Mounted on
devtmpfs                                              16308164           0    16308164   0% /dev
tmpfs                                                 16327196          84    16327112   1% /dev/shm
/dev/mapper/rhel-root                                 27249664     4659988    22589676  18% /
/dev/mapper/rhel-var                                  12572672     1761440    10811232  15% /var
/dev/sda1                                              1041288      285652      755636  28% /boot
eu-vfilerd-01-80-pb:/db_eu_hrznp_d003_binlogs01      179306496    96421504    82884992  54% /binlogs
eu-vfilerd-03-80-rb:/app_tst_800/orabackup_nonprod 28454158336 15711229120 12742929216  56% /backups_nonprod
`
	got := ParseDFSnapshot(input, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(got), got)
	}
	if got[0].Mount != "/backups_nonprod" || got[0].UsePct != 56 {
		t.Errorf("row 0 expected /backups_nonprod @56%%, got %+v", got[0])
	}
	if got[1].UsePct != 54 {
		t.Errorf("row 1 expected UsePct 54, got %d", got[1].UsePct)
	}
}

func TestParseDFSnapshot_UsesLastTSBlock(t *testing.T) {
	input := `TS 1 2026-01-01 00:00:00
Filesystem     1K-blocks    Used  Available Use% Mounted on
/dev/old          1000000   100000   900000  10% /old
TS 2 2026-01-01 00:00:01
Filesystem     1K-blocks    Used  Available Use% Mounted on
/dev/new          2000000  1800000  200000  90% /new
`
	got := ParseDFSnapshot(input, 5)
	if len(got) != 1 {
		t.Fatalf("expected 1 row from last block, got %d", len(got))
	}
	if got[0].Mount != "/new" || got[0].UsePct != 90 {
		t.Errorf("expected /new @90%%, got %+v", got[0])
	}
}
