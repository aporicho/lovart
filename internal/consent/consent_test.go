package consent

import "testing"

func TestValidatePaidConsentPass(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:              true,
		ConfirmedModel:           "lovart-v1",
		ConfirmedMode:            "generate",
		ConfirmedMaxCredits:      3,
		ConfirmedMaxTotalCredits: 9,
		ConfirmedOutputs:         4,
		ConfirmedRemoteRequests:  2,
		ConfirmedMaxActiveTasks:  1,
	}, PaidScope{
		Model:          "lovart-v1",
		Mode:           "generate",
		QuoteCredits:   2.5,
		TotalCredits:   8,
		Outputs:        4,
		RemoteRequests: 2,
		MaxActiveTasks: 1,
	})

	assertAllowed(t, result)
}

func TestValidatePaidConsentMissingStructuredConfirmation(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{}, PaidScope{QuoteCredits: 1})

	assertDenied(t, result, CodeStructuredConsentMissing)
}

func TestValidatePaidConsentModelMismatch(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:    true,
		ConfirmedModel: "lovart-v1",
	}, PaidScope{Model: "lovart-v2"})

	assertDenied(t, result, CodeStructuredConsentModelMismatch)
}

func TestValidatePaidConsentModeMismatch(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:   true,
		ConfirmedMode: "generate",
	}, PaidScope{Mode: "jobs"})

	assertDenied(t, result, CodeStructuredConsentModeMismatch)
}

func TestValidatePaidConsentCreditExceeded(t *testing.T) {
	tests := []struct {
		name    string
		consent PaidConsent
		scope   PaidScope
	}{
		{
			name: "single quote credits",
			consent: PaidConsent{
				ConfirmPaid:         true,
				ConfirmedMaxCredits: 1,
			},
			scope: PaidScope{QuoteCredits: 1.01},
		},
		{
			name: "batch total credits",
			consent: PaidConsent{
				ConfirmPaid:              true,
				ConfirmedMaxTotalCredits: 10,
			},
			scope: PaidScope{TotalCredits: 10.01},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePaidConsent(tt.consent, tt.scope)
			assertDenied(t, result, CodeStructuredConsentCreditLimitExceeded)
		})
	}
}

func TestValidatePaidConsentOutputExceeded(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:      true,
		ConfirmedOutputs: 2,
	}, PaidScope{Outputs: 3})

	assertDenied(t, result, CodeStructuredConsentOutputLimitExceeded)
}

func TestValidatePaidConsentRemoteRequestsExceeded(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:             true,
		ConfirmedRemoteRequests: 1,
	}, PaidScope{RemoteRequests: 2})

	assertDenied(t, result, CodeStructuredConsentRemoteLimitExceeded)
}

func TestValidatePaidConsentMaxActiveTasksExceeded(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:             true,
		ConfirmedMaxActiveTasks: 2,
	}, PaidScope{MaxActiveTasks: 3})

	assertDenied(t, result, CodeStructuredConsentActiveTaskExceeded)
}

func TestValidatePaidConsentZeroValuesAreNotLimits(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{
		ConfirmPaid:              true,
		ConfirmedMaxCredits:      0,
		ConfirmedMaxTotalCredits: 0,
		ConfirmedOutputs:         0,
		ConfirmedRemoteRequests:  0,
		ConfirmedMaxActiveTasks:  0,
	}, PaidScope{
		QuoteCredits:   100,
		TotalCredits:   200,
		Outputs:        50,
		RemoteRequests: 25,
		MaxActiveTasks: 10,
	})

	assertAllowed(t, result)
}

func TestValidatePaidConsentBackwardCompatibleOptionalFields(t *testing.T) {
	result := ValidatePaidConsent(PaidConsent{ConfirmPaid: true}, PaidScope{
		Model:          "future-model",
		Mode:           "jobs",
		QuoteCredits:   1.5,
		TotalCredits:   6,
		Outputs:        8,
		RemoteRequests: 4,
		MaxActiveTasks: 2,
	})

	assertAllowed(t, result)
}

func assertAllowed(t *testing.T, result ValidationResult) {
	t.Helper()
	if !result.Allowed {
		t.Fatalf("expected allowed result, got denied code=%q message=%q", result.Code, result.Message)
	}
	if result.Code != CodeOK {
		t.Fatalf("expected empty code for allowed result, got %q", result.Code)
	}
}

func assertDenied(t *testing.T, result ValidationResult, code DenialCode) {
	t.Helper()
	if result.Allowed {
		t.Fatalf("expected denied result")
	}
	if result.Code != code {
		t.Fatalf("code: got %q want %q (message=%q)", result.Code, code, result.Message)
	}
	if result.Message == "" {
		t.Fatalf("expected denial message")
	}
}
