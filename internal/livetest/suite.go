package livetest

import (
	"context"
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// TestCase defines a single test scenario in the live test suite.
type TestCase interface {
	Name() string
	Category() string
	Description() string
	RequiresInteractive() bool
	RequiresRecipient() bool
	Run(ctx context.Context, s *Suite) (*TestRecord, error)
}

// Suite orchestrates and executes live test cases against MAX Bot API.
type Suite struct {
	Config     *Config
	Client     *maxbot.Client
	BotUser    *maxbot.User
	Cases      []TestCase
	Records    []*TestRecord
	OnProgress func(rec *TestRecord)
}

// NewSuite creates a new test suite initialized with client and configuration.
func NewSuite(cfg *Config) (*Suite, error) {
	if cfg == nil {
		cfg = LoadConfig()
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("MAX_BOT_TOKEN is required to initialize live test suite")
	}

	client, err := maxbot.NewClient(cfg.Token, maxbot.WithBaseURL(cfg.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create MAX Bot client: %w", err)
	}

	return &Suite{
		Config: cfg,
		Client: client,
		Cases:  make([]TestCase, 0),
	}, nil
}

// Register adds one or more test cases to the suite.
func (s *Suite) Register(cases ...TestCase) {
	s.Cases = append(s.Cases, cases...)
}

// RunAll executes all registered test cases sequentially, enforcing lifecycle and requirements.
func (s *Suite) RunAll(ctx context.Context) ([]*TestRecord, SuiteSummary) {
	startTime := time.Now()
	s.Records = make([]*TestRecord, 0, len(s.Cases))
	summary := SuiteSummary{
		Total:     len(s.Cases),
		StartTime: startTime,
	}

	for _, tc := range s.Cases {
		record := &TestRecord{
			Name:        tc.Name(),
			Category:    tc.Category(),
			Description: tc.Description(),
			Details:     make(map[string]any),
		}

		// Check if interactive is required but disabled
		if tc.RequiresInteractive() && !s.Config.Interactive {
			record.Status = StatusSkipped
			record.ErrorMsg = "requires interactive mode (--interactive)"
			summary.Skipped++
			s.Records = append(s.Records, record)
			if s.OnProgress != nil {
				s.OnProgress(record)
			}
			continue
		}

		// Check if recipient is required but unset
		if tc.RequiresRecipient() && s.Config.TestUserID == 0 && s.Config.TestChatID == 0 {
			record.Status = StatusSkipped
			record.ErrorMsg = "requires TEST_USER_ID or TEST_CHAT_ID"
			summary.Skipped++
			s.Records = append(s.Records, record)
			if s.OnProgress != nil {
				s.OnProgress(record)
			}
			continue
		}

		// Run the test
		caseStart := time.Now()
		res, err := tc.Run(ctx, s)
		record.Duration = time.Since(caseStart)

		if res != nil {
			record.Status = res.Status
			record.RTT = res.RTT
			record.Error = res.Error
			if res.Error != nil {
				record.ErrorMsg = res.Error.Error()
			}
			record.Events = res.Events
			for k, v := range res.Details {
				record.Details[k] = v
			}
		}

		if err != nil && record.Status == "" {
			record.Status = StatusFailed
			record.Error = err
			record.ErrorMsg = err.Error()
		}

		switch record.Status {
		case StatusPassed:
			summary.Passed++
		case StatusTimeout:
			summary.Timeout++
		case StatusSkipped:
			summary.Skipped++
		default:
			record.Status = StatusFailed
			summary.Failed++
		}

		s.Records = append(s.Records, record)
		if s.OnProgress != nil {
			s.OnProgress(record)
		}
	}

	summary.EndTime = time.Now()
	summary.Duration = summary.EndTime.Sub(startTime)
	return s.Records, summary
}
