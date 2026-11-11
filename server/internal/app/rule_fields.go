package app

import "github.com/Jinnrry/pmail/dto/parsemail"

func ruleBuildUsers(users []*parsemail.User) string {
	ret := ""
	for i, u := range users {
		if i != 0 {
			ret += ","
		}
		ret += u.EmailAddress
	}
	return ret
}

func ruleGetFieldContent(field string, email *parsemail.Email) string {
	switch field {
	case "ReplyTo":
		return ruleBuildUsers(email.ReplyTo)
	case "From":
		return email.From.EmailAddress
	case "Subject":
		return email.Subject
	case "To":
		return ruleBuildUsers(email.To)
	case "Bcc":
		return ruleBuildUsers(email.Bcc)
	case "Cc":
		return ruleBuildUsers(email.Cc)
	case "Text":
		return string(email.Text)
	case "Html":
		return string(email.HTML)
	case "Sender":
		return email.Sender.EmailAddress
	case "Content":
		b := string(email.HTML)
		b2 := string(email.Text)
		return b + b2
	}
	return ""
}
