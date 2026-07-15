package migrate

import "testing"

func TestClassifyMigrationEvidence(t *testing.T) {
	tests := []struct {
		name     string
		evidence migrationRepositoryEvidence
		want     migrationRecoveryClassification
	}{
		{name: "ready", evidence: migrationRepositoryEvidence{sourceExists: true}, want: migrationReady},
		{name: "partial journal wins over file shape", evidence: migrationRepositoryEvidence{journalExists: true, sourceExists: true, configExists: true}, want: migrationPartiallyApplied},
		{name: "already complete", evidence: migrationRepositoryEvidence{configExists: true, stateExists: true}, want: migrationAlreadyComplete},
		{name: "config only", evidence: migrationRepositoryEvidence{configExists: true}, want: migrationIncompleteTarget},
		{name: "state only", evidence: migrationRepositoryEvidence{stateExists: true}, want: migrationIncompleteTarget},
		{name: "source target conflict", evidence: migrationRepositoryEvidence{sourceExists: true, configExists: true, stateExists: true}, want: migrationSourceTargetConflict},
		{name: "missing", evidence: migrationRepositoryEvidence{}, want: migrationSourceMissing},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyMigrationEvidence(test.evidence); got != test.want {
				t.Fatalf("classification = %d, want %d", got, test.want)
			}
		})
	}
}

func TestSelectMigrationPlanningOperation(t *testing.T) {
	tests := []struct {
		classification migrationRecoveryClassification
		want           migrationPlanningOperation
	}{
		{classification: migrationReady, want: planNewMigration},
		{classification: migrationPartiallyApplied, want: planInterruptedMigration},
		{classification: migrationAlreadyComplete, want: returnCompletedMigration},
		{classification: migrationSourceMissing, want: inspectUnsupportedMigrationSource},
		{classification: migrationIncompleteTarget, want: refuseIncompleteMigrationTarget},
		{classification: migrationSourceTargetConflict, want: refuseMigrationSourceTargetConflict},
	}

	for _, test := range tests {
		got, err := selectMigrationPlanningOperation(test.classification)
		if err != nil {
			t.Fatalf("select classification %d: %v", test.classification, err)
		}
		if got != test.want {
			t.Fatalf("classification %d operation = %d, want %d", test.classification, got, test.want)
		}
	}

	if _, err := selectMigrationPlanningOperation(0); err == nil {
		t.Fatal("unknown classification was accepted")
	}
}

func TestSelectRecoveryFileOperations(t *testing.T) {
	if got := selectRecoveryTargetOperation(false, false); got != persistMigrationTarget {
		t.Fatalf("missing target operation = %d, want persist", got)
	}
	if got := selectRecoveryTargetOperation(true, false); got != persistMigrationTarget {
		t.Fatalf("partial target operation = %d, want persist", got)
	}
	if got := selectRecoveryTargetOperation(true, true); got != retainMigrationTarget {
		t.Fatalf("complete target operation = %d, want retain", got)
	}
	if got := selectRecoverySourceOperation(true); got != archiveMigrationSource {
		t.Fatalf("active source operation = %d, want archive", got)
	}
	if got := selectRecoverySourceOperation(false); got != retainArchivedMigrationSource {
		t.Fatalf("archived source operation = %d, want retain", got)
	}
}
