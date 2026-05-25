// Command ironflyer-smoke is the end-to-end happy-path runner: signs
// in (or signs up), checks the wallet, optionally starts a top-up,
// fires DescribeIdea, polls the execution to a terminal status, and
// prints the executionSupportBundle when the run succeeded.
//
// Designed to be hand-driven from a developer's shell during the V22
// closeout — the output uses plain ANSI codes (no charm/lipgloss dep
// is added to the orchestrator module) and the flow is intentionally
// linear. Polls instead of subscribing to graphql-ws so the runner
// stays a single compilation unit with no extra deps.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ANSI colour helpers — kept inline so the smoke runner has no
// runtime deps beyond stdlib.
const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
)

func step(idx int, label string) {
	fmt.Printf("\n%s%s[step %d]%s %s%s%s\n", cBold, cBlue, idx, cReset, cCyan, label, cReset)
}

func okf(format string, args ...any) {
	fmt.Printf("  %s✓%s ", cGreen, cReset)
	fmt.Printf(format+"\n", args...)
}

func warnf(format string, args ...any) {
	fmt.Printf("  %s!%s ", cYellow, cReset)
	fmt.Printf(format+"\n", args...)
}

func failf(format string, args ...any) {
	fmt.Printf("  %s✗%s ", cRed, cReset)
	fmt.Printf(format+"\n", args...)
}

func dimf(format string, args ...any) {
	fmt.Printf("    %s", cDim)
	fmt.Printf(format, args...)
	fmt.Printf("%s\n", cReset)
}

type smokeClient struct {
	endpoint string
	token    string
	http     *http.Client
}

func newClient(endpoint string, timeout time.Duration) *smokeClient {
	return &smokeClient{
		endpoint: endpoint,
		http:     &http.Client{Timeout: timeout},
	}
}

type gqlError struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

type gqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors,omitempty"`
}

func (c *smokeClient) do(ctx context.Context, query string, variables map[string]any, out any) error {
	body, _ := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var envelope gqlResp
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode response: %w (body: %s)", err, string(raw))
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", envelope.Errors[0].Message)
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}
	return nil
}

type signInData struct {
	SignIn struct {
		Token string `json:"token"`
		User  struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"signIn"`
}

type signUpData struct {
	SignUp struct {
		Token string `json:"token"`
		User  struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"signUp"`
}

type walletData struct {
	Wallet struct {
		BalanceUSD       float64 `json:"balanceUSD"`
		HoldUSD          float64 `json:"holdUSD"`
		AvailableUSD     float64 `json:"availableUSD"`
		LifetimeTopUpUSD float64 `json:"lifetimeTopUpUSD"`
	} `json:"wallet"`
}

type topUpData struct {
	WalletCreateTopUp struct {
		URL       string `json:"url"`
		SessionID string `json:"sessionID"`
	} `json:"walletCreateTopUp"`
}

type describeIdeaData struct {
	DescribeIdea struct {
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Execution struct {
			ID            string  `json:"id"`
			Status        string  `json:"status"`
			BudgetUSD     float64 `json:"budgetUSD"`
			PromptSummary string  `json:"promptSummary"`
		} `json:"execution"`
		Idea struct {
			Title           string   `json:"title"`
			BlueprintID     string   `json:"blueprintID"`
			BlueprintReason string   `json:"blueprintReason"`
			Tags            []string `json:"tags"`
			Confidence      float64  `json:"confidence"`
		} `json:"idea"`
		CostEstimate struct {
			LowUSD    float64 `json:"lowUSD"`
			MedianUSD float64 `json:"medianUSD"`
			HighUSD   float64 `json:"highUSD"`
			P95USD    float64 `json:"p95USD"`
		} `json:"costEstimate"`
	} `json:"describeIdea"`
}

type executionStatusData struct {
	Execution struct {
		ID            string  `json:"id"`
		Status        string  `json:"status"`
		BudgetUSD     float64 `json:"budgetUSD"`
		SpentUSD      float64 `json:"spentUSD"`
		ReservedUSD   float64 `json:"reservedUSD"`
		PromptSummary string  `json:"promptSummary"`
	} `json:"execution"`
}

type supportBundleData struct {
	ExecutionSupportBundle struct {
		ExecutionID   string   `json:"executionID"`
		Status        string   `json:"status"`
		PreviewURL    *string  `json:"previewURL"`
		ProductionURL *string  `json:"productionURL"`
		ChangedFiles  []string `json:"changedFiles"`
		PatchCount    int      `json:"patchCount"`
		CostReport    struct {
			ProviderCostUSD float64 `json:"providerCostUSD"`
			SandboxCostUSD  float64 `json:"sandboxCostUSD"`
			TotalSpentUSD   float64 `json:"totalSpentUSD"`
		} `json:"costReport"`
		GateReport struct {
			CompletionScore float64 `json:"completionScore"`
			Stages          []struct {
				Name        string `json:"name"`
				Status      string `json:"status"`
				IssuesCount int    `json:"issuesCount"`
			} `json:"stages"`
		} `json:"gateReport"`
	} `json:"executionSupportBundle"`
}

const (
	mutationSignUp = `mutation SignUp($input: SignUpInput!) {
		signUp(input: $input) { token user { id email } }
	}`
	mutationSignIn = `mutation SignIn($input: SignInInput!) {
		signIn(input: $input) { token user { id email } }
	}`
	queryWallet = `query Wallet {
		wallet { balanceUSD holdUSD availableUSD lifetimeTopUpUSD }
	}`
	mutationTopUp = `mutation TopUp($amount: Float!) {
		walletCreateTopUp(amountUSD: $amount) { url sessionID }
	}`
	mutationDescribeIdea = `mutation DescribeIdea($input: DescribeIdeaInput!) {
		describeIdea(input: $input) {
			project { id name }
			execution { id status budgetUSD promptSummary }
			idea { title blueprintID blueprintReason tags confidence }
			costEstimate { lowUSD medianUSD highUSD p95USD }
		}
	}`
	queryExecution = `query Execution($id: ID!) {
		execution(id: $id) {
			id status budgetUSD spentUSD reservedUSD promptSummary
		}
	}`
	querySupportBundle = `query SupportBundle($id: ID!) {
		executionSupportBundle(executionID: $id) {
			executionID status previewURL productionURL changedFiles patchCount
			costReport { providerCostUSD sandboxCostUSD totalSpentUSD }
			gateReport {
				completionScore
				stages { name status issuesCount }
			}
		}
	}`
)

func isTerminal(status string) bool {
	switch strings.ToLower(status) {
	case "succeeded", "failed", "stopped", "killed", "refunded":
		return true
	}
	return false
}

func main() {
	endpoint := flag.String("graphql", "http://localhost:8080/graphql", "GraphQL endpoint")
	email := flag.String("email", "demo@ironflyer.dev", "User email")
	password := flag.String("password", "demo1234", "User password")
	prompt := flag.String("prompt", "A landing page for a Pilates studio", "Product idea prompt")
	budget := flag.Float64("budget", 5.0, "Wallet budget hold in USD")
	topUpAmount := flag.Float64("topup", 25.0, "Top-up amount in USD if wallet is short (set 0 to skip)")
	timeout := flag.Duration("timeout", 10*time.Minute, "End-to-end timeout")
	pollInterval := flag.Duration("poll", 5*time.Second, "Status poll interval")
	skipSignup := flag.Bool("skip-signup", false, "Skip signUp attempt (assume user exists)")
	autoConfirmTopUp := flag.Bool("auto-confirm-topup", false, "Don't pause for manual checkout; treat top-up as best-effort")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Printf("%s%sIronflyer end-to-end smoke runner%s\n", cBold, cBlue, cReset)
	dimf("endpoint: %s", *endpoint)
	dimf("user:     %s", *email)
	dimf("budget:   $%.2f", *budget)
	dimf("timeout:  %s", timeout.String())

	c := newClient(*endpoint, 30*time.Second)

	// Step 1 — authenticate (signUp + fall back to signIn).
	step(1, "Authenticate")
	if !*skipSignup {
		var sup signUpData
		if err := c.do(ctx, mutationSignUp, map[string]any{
			"input": map[string]any{
				"email":    *email,
				"password": *password,
			},
		}, &sup); err != nil {
			warnf("signUp failed (likely existing user): %v", err)
		} else {
			c.token = sup.SignUp.Token
			okf("signed up as %s (id=%s)", sup.SignUp.User.Email, sup.SignUp.User.ID)
		}
	}
	if c.token == "" {
		var sin signInData
		if err := c.do(ctx, mutationSignIn, map[string]any{
			"input": map[string]any{
				"email":    *email,
				"password": *password,
			},
		}, &sin); err != nil {
			fail("signIn", err)
			os.Exit(1)
		}
		c.token = sin.SignIn.Token
		okf("signed in as %s (id=%s)", sin.SignIn.User.Email, sin.SignIn.User.ID)
	}

	// Step 2 — wallet snapshot.
	step(2, "Wallet snapshot")
	var wallet walletData
	if err := c.do(ctx, queryWallet, nil, &wallet); err != nil {
		fail("wallet query", err)
		os.Exit(1)
	}
	okf("balance=$%.2f hold=$%.2f available=$%.2f lifetime=$%.2f",
		wallet.Wallet.BalanceUSD, wallet.Wallet.HoldUSD, wallet.Wallet.AvailableUSD, wallet.Wallet.LifetimeTopUpUSD)

	// Step 3 — top up if needed.
	if wallet.Wallet.AvailableUSD < *budget && *topUpAmount > 0 {
		step(3, fmt.Sprintf("Top up wallet ($%.2f)", *topUpAmount))
		var top topUpData
		if err := c.do(ctx, mutationTopUp, map[string]any{"amount": *topUpAmount}, &top); err != nil {
			warnf("walletCreateTopUp failed (Stripe likely unwired): %v", err)
			warnf("continuing — describeIdea will succeed only if available >= budget")
		} else {
			okf("checkout url: %s", top.WalletCreateTopUp.URL)
			if !*autoConfirmTopUp {
				fmt.Printf("\n  %sOpen the URL above, pay with test card 4242 4242 4242 4242, then press <enter> to continue...%s ", cYellow, cReset)
				_, _ = fmt.Scanln()
				// Re-fetch wallet.
				if err := c.do(ctx, queryWallet, nil, &wallet); err == nil {
					okf("wallet after topup: available=$%.2f", wallet.Wallet.AvailableUSD)
				}
			}
		}
	} else if wallet.Wallet.AvailableUSD < *budget {
		warnf("wallet available ($%.2f) below requested budget ($%.2f) and topup=0 — describeIdea will clamp", wallet.Wallet.AvailableUSD, *budget)
	}

	// Step 4 — describeIdea.
	step(4, "DescribeIdea — book a paid execution")
	var di describeIdeaData
	if err := c.do(ctx, mutationDescribeIdea, map[string]any{
		"input": map[string]any{
			"text":              *prompt,
			"budgetUSDOverride": *budget,
			"startImmediately":  true,
		},
	}, &di); err != nil {
		fail("describeIdea", err)
		os.Exit(1)
	}
	okf("project=%s (%s)", di.DescribeIdea.Project.Name, di.DescribeIdea.Project.ID)
	okf("execution=%s status=%s budget=$%.2f", di.DescribeIdea.Execution.ID, di.DescribeIdea.Execution.Status, di.DescribeIdea.Execution.BudgetUSD)
	okf("idea: %s (blueprint=%s, conf=%.2f)", di.DescribeIdea.Idea.Title, di.DescribeIdea.Idea.BlueprintID, di.DescribeIdea.Idea.Confidence)
	dimf("reason: %s", di.DescribeIdea.Idea.BlueprintReason)
	dimf("estimate: low=$%.2f median=$%.2f high=$%.2f p95=$%.2f",
		di.DescribeIdea.CostEstimate.LowUSD, di.DescribeIdea.CostEstimate.MedianUSD,
		di.DescribeIdea.CostEstimate.HighUSD, di.DescribeIdea.CostEstimate.P95USD)
	if len(di.DescribeIdea.Idea.Tags) > 0 {
		dimf("tags: %s", strings.Join(di.DescribeIdea.Idea.Tags, ", "))
	}

	execID := di.DescribeIdea.Execution.ID

	// Step 5 — poll execution.
	step(5, "Poll execution until terminal")
	lastStatus := ""
	for {
		select {
		case <-ctx.Done():
			fail("poll", ctx.Err())
			os.Exit(1)
		default:
		}
		var es executionStatusData
		if err := c.do(ctx, queryExecution, map[string]any{"id": execID}, &es); err != nil {
			warnf("execution poll: %v", err)
		} else {
			if es.Execution.Status != lastStatus {
				okf("status=%s spent=$%.2f reserved=$%.2f", es.Execution.Status, es.Execution.SpentUSD, es.Execution.ReservedUSD)
				lastStatus = es.Execution.Status
			} else {
				dimf("status=%s spent=$%.2f", es.Execution.Status, es.Execution.SpentUSD)
			}
			if isTerminal(es.Execution.Status) {
				break
			}
		}
		select {
		case <-ctx.Done():
			fail("poll", ctx.Err())
			os.Exit(1)
		case <-time.After(*pollInterval):
		}
	}

	// Step 6 — support bundle.
	step(6, "Fetch executionSupportBundle")
	var sb supportBundleData
	if err := c.do(ctx, querySupportBundle, map[string]any{"id": execID}, &sb); err != nil {
		warnf("supportBundle: %v", err)
	} else {
		okf("status=%s patches=%d files=%d completion=%.2f",
			sb.ExecutionSupportBundle.Status,
			sb.ExecutionSupportBundle.PatchCount,
			len(sb.ExecutionSupportBundle.ChangedFiles),
			sb.ExecutionSupportBundle.GateReport.CompletionScore)
		if sb.ExecutionSupportBundle.PreviewURL != nil {
			okf("preview: %s", *sb.ExecutionSupportBundle.PreviewURL)
		}
		if sb.ExecutionSupportBundle.ProductionURL != nil {
			okf("production: %s", *sb.ExecutionSupportBundle.ProductionURL)
		}
		okf("cost: provider=$%.4f sandbox=$%.4f total=$%.4f",
			sb.ExecutionSupportBundle.CostReport.ProviderCostUSD,
			sb.ExecutionSupportBundle.CostReport.SandboxCostUSD,
			sb.ExecutionSupportBundle.CostReport.TotalSpentUSD)
		for _, st := range sb.ExecutionSupportBundle.GateReport.Stages {
			dimf("gate %-24s %-12s issues=%d", st.Name, st.Status, st.IssuesCount)
		}
	}

	// Exit code.
	finalStatus := strings.ToLower(lastStatus)
	if finalStatus == "succeeded" {
		fmt.Printf("\n%s%s✓ smoke run completed (status=%s)%s\n", cBold, cGreen, lastStatus, cReset)
		return
	}
	fmt.Printf("\n%s%s✗ smoke run terminal status=%s%s\n", cBold, cRed, lastStatus, cReset)
	os.Exit(1)
}

func fail(label string, err error) {
	failf("%s: %v", label, err)
}
