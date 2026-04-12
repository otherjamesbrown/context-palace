package client

import "testing"

func TestValidateScheduleWorkflowType(t *testing.T) {
	valid := []string{
		ScheduleWorkflowDriftScan,
		ScheduleWorkflowCanary,
		ScheduleWorkflowTriage,
	}

	for _, workflowType := range valid {
		if err := ValidateScheduleWorkflowType(workflowType); err != nil {
			t.Fatalf("expected %q to be valid, got error: %v", workflowType, err)
		}
	}
}

func TestValidateScheduleWorkflowType_Invalid(t *testing.T) {
	if err := ValidateScheduleWorkflowType("nightly"); err == nil {
		t.Fatal("expected invalid workflow type to return error")
	}
}
