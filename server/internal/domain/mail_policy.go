package domain

import "strings"

// CanAccessEmail preserves administrator access and per-user ownership checks.
func CanAccessEmail(isAdmin bool, ownerCount int) bool { return isAdmin || ownerCount > 0 }

// IsSpoofedSender identifies a local-domain sender whose SPF verification failed.
func IsSpoofedSender(localSender, spfPassed bool) bool { return localSender && !spfPassed }

// ReceiveStatus preserves spoofing precedence over plugin classifications.
func ReceiveStatus(pluginStatus, forgedStatus int, spoofed bool) int {
	if spoofed {
		return forgedStatus
	}
	return pluginStatus
}

func ShouldDropUnknownRecipient(enabled bool, level int, spfPassed, dkimPassed bool) bool {
	return enabled && ((level == 1 && !spfPassed && !dkimPassed) || (level == 2 && !spfPassed) || (level == 3 && !dkimPassed))
}

func IsSelfForward(account, recipient string) bool { return strings.EqualFold(account, recipient) }

// SenderAllowed applies the existing administrator and account ownership policy.
func SenderAllowed(isAdmin bool, account, sender string) bool { return isAdmin || account == sender }
