package command

import "testing"

func TestCommandPolicy_AcceptMode_UnmatchedIsDenied(t *testing.T) {
	p := NewCommandPolicy(PolicyModeAccept, nil)

	d := p.Evaluate(Command{Program: "rm", Args: []string{"-rf", "/"}})
	if d.Allowed {
		t.Fatalf("expected accept-mode with no rules to deny everything, got allowed with reason %q", d.Reason)
	}
}

func TestCommandPolicy_AcceptMode_ProgramOnlyRuleAllowsAnyArgs(t *testing.T) {
	p := NewCommandPolicy(PolicyModeAccept, []Rule{
		{Program: "ls", Allowed: true},
	})

	d := p.Evaluate(Command{Program: "ls", Args: []string{"-la", "/tmp"}})
	if !d.Allowed {
		t.Fatalf("expected a program-only rule to allow any args, got denied: %s", d.Reason)
	}
}

func TestCommandPolicy_AcceptMode_ArgsPrefixMatching(t *testing.T) {
	p := NewCommandPolicy(PolicyModeAccept, []Rule{
		{Program: "git", ArgsPrefixes: [][]string{{"status"}, {"log"}}, Allowed: true},
	})

	allowed := Command{Program: "git", Args: []string{"status"}}
	if d := p.Evaluate(allowed); !d.Allowed {
		t.Fatalf("expected git status to be allowed, got denied: %s", d.Reason)
	}

	denied := Command{Program: "git", Args: []string{"push", "--force"}}
	if d := p.Evaluate(denied); d.Allowed {
		t.Fatal("expected git push --force to be denied — it doesn't match either allowed prefix")
	}
}

func TestCommandPolicy_ProhibitedMode_UnmatchedIsAllowed(t *testing.T) {
	p := NewCommandPolicy(PolicyModeProhibited, []Rule{
		{Program: "rm", Allowed: false},
	})

	d := p.Evaluate(Command{Program: "ls"})
	if !d.Allowed {
		t.Fatalf("expected prohibited-mode to allow anything not explicitly denied, got denied: %s", d.Reason)
	}
}

func TestCommandPolicy_ProhibitedMode_MatchedRuleDenies(t *testing.T) {
	p := NewCommandPolicy(PolicyModeProhibited, []Rule{
		{Program: "rm", Allowed: false},
	})

	d := p.Evaluate(Command{Program: "rm", Args: []string{"-rf", "/"}})
	if d.Allowed {
		t.Fatal("expected rm to be denied by the prohibited rule regardless of args (program-only rule)")
	}
}

func TestCommandPolicy_ArgsPrefix_ShorterThanPrefixDoesNotMatch(t *testing.T) {
	p := NewCommandPolicy(PolicyModeAccept, []Rule{
		{Program: "docker", ArgsPrefixes: [][]string{{"system", "prune"}}, Allowed: true},
	})

	d := p.Evaluate(Command{Program: "docker", Args: []string{"system"}})
	if d.Allowed {
		t.Fatal("expected args shorter than the prefix to not match")
	}
}
