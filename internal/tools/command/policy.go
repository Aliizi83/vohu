package command

type Decision struct {
	Allowed bool
	Reason  string
}

type PolicyMode string

const (
	PolicyModeAccept     PolicyMode = "accept"
	PolicyModeProhibited PolicyMode = "prohibited"
)

type Policy interface {
	Evaluate(command Command) Decision
}

type Rule struct {
	Program      string
	ArgsPrefixes [][]string
	Allowed      bool
}

type CommandPolicy struct {
	rules []Rule
	mode  PolicyMode
}

func NewCommandPolicy(mode PolicyMode, rules []Rule) *CommandPolicy {
	return &CommandPolicy{
		rules: rules,
		mode:  mode,
	}
}

func (p *CommandPolicy) Evaluate(
	command Command,
) Decision {

	for _, rule := range p.rules {
		if command.Program != rule.Program {
			continue
		}

		if len(rule.ArgsPrefixes) == 0 {
			return Decision{
				Allowed: rule.Allowed,
				Reason:  "matched command rule",
			}
		}

		for _, prefix := range rule.ArgsPrefixes {
			if hasArgsPrefix(command.Args, prefix) {
				return Decision{
					Allowed: rule.Allowed,
					Reason:  "matched command rule",
				}
			}
		}
	}

	switch p.mode {
	case PolicyModeAccept:
		return Decision{
			Allowed: false,
			Reason:  "command is not accepted by policy",
		}

	case PolicyModeProhibited:
		return Decision{
			Allowed: true,
			Reason:  "command is not prohibited by policy",
		}

	default:
		return Decision{
			Allowed: false,
			Reason:  "unknown policy mode",
		}
	}
}

func hasArgsPrefix(
	args []string,
	prefix []string,
) bool {

	if len(args) < len(prefix) {
		return false
	}

	for i := range prefix {
		if args[i] != prefix[i] {
			return false
		}
	}

	return true
}
