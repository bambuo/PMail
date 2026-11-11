package domain

import "testing"

func TestMailOwnershipPolicy(t *testing.T) {
	for _, tt := range []struct {
		name   string
		admin  bool
		owners int
		want   bool
	}{
		{"owner", false, 1, true}, {"unrelated_user", false, 0, false}, {"administrator", true, 0, true}, {"multiple_owners", false, 2, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccessEmail(tt.admin, tt.owners); got != tt.want {
				t.Fatalf("CanAccessEmail = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSpoofingOverridesPluginStatus(t *testing.T) {
	for _, tt := range []struct {
		name                string
		local, spf, spoofed bool
	}{
		{"local_spf_failure", true, false, true}, {"local_spf_pass", true, true, false}, {"external_spf_failure", false, false, false}, {"external_spf_pass", false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spoofed := IsSpoofedSender(tt.local, tt.spf)
			if spoofed != tt.spoofed {
				t.Fatalf("spoofed = %t, want %t", spoofed, tt.spoofed)
			}
			for _, pluginStatus := range []int{0, 3, 5, 6} {
				want := pluginStatus
				if tt.spoofed {
					want = 3
				}
				if got := ReceiveStatus(pluginStatus, 3, spoofed); got != want {
					t.Fatalf("plugin status %d became %d, want %d", pluginStatus, got, want)
				}
			}
		})
	}
}

func TestUnknownRecipientSpamPolicy(t *testing.T) {
	// Outcomes are ordered as: both fail, SPF fails, DKIM fails, both pass.
	outcomes := []struct{ spf, dkim bool }{{false, false}, {false, true}, {true, false}, {true, true}}
	for _, tt := range []struct {
		level int
		want  [4]bool
	}{
		{-1, [4]bool{}}, {0, [4]bool{}}, {1, [4]bool{true, false, false, false}},
		{2, [4]bool{true, true, false, false}}, {3, [4]bool{true, false, true, false}}, {4, [4]bool{}},
	} {
		for i, result := range outcomes {
			if got := ShouldDropUnknownRecipient(true, tt.level, result.spf, result.dkim); got != tt.want[i] {
				t.Fatalf("level=%d SPF=%t DKIM=%t: drop=%t, want %t", tt.level, result.spf, result.dkim, got, tt.want[i])
			}
			if ShouldDropUnknownRecipient(false, tt.level, result.spf, result.dkim) {
				t.Fatalf("disabled dropping rejected mail at level %d", tt.level)
			}
		}
	}
}

func TestSenderOwnershipPolicy(t *testing.T) {
	for _, tt := range []struct {
		admin           bool
		account, sender string
		want            bool
	}{
		{false, "alice", "alice", true}, {false, "alice", "bob", false}, {false, "alice", "Alice", false}, {true, "alice", "bob", true},
	} {
		if got := SenderAllowed(tt.admin, tt.account, tt.sender); got != tt.want {
			t.Fatalf("admin=%t account=%q sender=%q: allowed=%t, want %t", tt.admin, tt.account, tt.sender, got, tt.want)
		}
	}
}

func TestSelfForwardingIsCaseInsensitive(t *testing.T) {
	if !IsSelfForward("Alice", "alice") {
		t.Fatal("case variants must remain self-forwarding")
	}
	if IsSelfForward("alice", "bob") {
		t.Fatal("different recipients must remain allowed")
	}
}

func TestRuleMatchersPreserveMatchingSemantics(t *testing.T) {
	for _, tt := range []struct {
		name    string
		matcher RuleMatch
		content string
		want    bool
	}{
		{"contains", NewContainsRule("Subject", "offer"), "an offer", true},
		{"contains_case_sensitive", NewContainsRule("Subject", "offer"), "OFFER", false},
		{"equal", NewEqualRule("Subject", "offer"), "offer", true},
		{"equal_not_substring", NewEqualRule("Subject", "offer"), "an offer", false},
		{"regex_negative_lookahead_rejects", NewRegexRule("From", `^(?!.*abc\.com).*`), "aa@abc.com", false},
		{"regex_negative_lookahead_accepts", NewRegexRule("From", `^(?!.*abc\.com).*`), "aa@example.test", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.matcher.Match(tt.content)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("matched=%t, want %t", got, tt.want)
			}
		})
	}
}
