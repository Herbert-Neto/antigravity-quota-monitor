//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

const sampleUsageJSON = `{"conversation_id":"","status":"SUCCESS","command":{"name":"usage","data":{"description":"Limits info","groups":[{"name":"Gemini Models","description":"Models within this group: Gemini 4 Hydrogen, Gemini 3.8 Flash","buckets":[{"id":"gemini-weekly","name":"Weekly Limit Remaining","description":"You have used some of your weekly limit","window":"weekly","remaining_fraction":0.8842562437057495,"reset_time":"2026-11-21T02:51:17Z"},{"id":"gemini-5h","name":"Five Hour Limit Remaining","description":"You have used some of your 5-hour limit","window":"5h","remaining_fraction":0.9323266744613647,"reset_time":"2026-11-18T00:49:54Z"}]},{"name":"Claude and GPT models","description":"Models within this group: Claude Opus 5.5, Claude Sonnet 5.5, GPT-OSS 120B","buckets":[{"id":"3p-weekly","name":"Weekly Limit Remaining","window":"weekly","remaining_fraction":1,"reset_time":"2026-11-24T20:27:13Z"},{"id":"3p-5h","name":"Five Hour Limit Remaining","window":"5h","remaining_fraction":1,"reset_time":"2026-11-18T01:27:13Z"}]},{"name":"Gemini 4 Argon","description":"For paid Pro plans, Argon quota is separate.","buckets":[{"id":"gemini-argon-wtu-weekly","name":"Gemini 4 Argon Weekly Limit","description":"You have used some of your weekly limit","window":"weekly","remaining_fraction":0.6960762739181519,"reset_time":"2026-11-23T21:51:08Z"}]}]}}}`

func TestParseUsageJSON(t *testing.T) {
	t.Run("valid full payload", func(t *testing.T) {
		out, err := parseUsageJSON([]byte(sampleUsageJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if out.Status != "SUCCESS" {
			t.Fatalf("expected status SUCCESS, got %q", out.Status)
		}
		if len(out.Command.Data.Groups) != 3 {
			t.Fatalf("expected 3 groups, got %d", len(out.Command.Data.Groups))
		}
		first := out.Command.Data.Groups[0].Buckets[0]
		if first.Window != "weekly" || math.Abs(first.RemainingFraction-0.8842562437057495) > 1e-9 {
			t.Fatalf("unexpected first bucket: %+v", first)
		}
		if first.ResetTime != "2026-11-21T02:51:17Z" {
			t.Fatalf("unexpected reset_time: %q", first.ResetTime)
		}
	})

	t.Run("prefixed log lines before JSON", func(t *testing.T) {
		raw := "Fetching usage...\nDEBUG something\n" + sampleUsageJSON + "\n"
		out, err := parseUsageJSON([]byte(raw))
		if err != nil {
			t.Fatalf("expected no error with prefixed logs, got %v", err)
		}
		if len(out.Command.Data.Groups) != 3 {
			t.Fatalf("expected 3 groups, got %d", len(out.Command.Data.Groups))
		}
	})

	t.Run("empty output", func(t *testing.T) {
		if _, err := parseUsageJSON([]byte("   \n\t ")); err == nil {
			t.Fatal("expected error for empty output")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		if _, err := parseUsageJSON([]byte(`{"status":"SUCCESS","command":`)); err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})
}

func TestLowestFraction(t *testing.T) {
	if got := LowestFraction(nil); got != -1 {
		t.Fatalf("expected -1 for nil data, got %v", got)
	}
	if got := LowestFraction(&UsageData{}); got != -1 {
		t.Fatalf("expected -1 for empty groups, got %v", got)
	}

	data := &UsageData{
		Groups: []QuotaGroup{
			{Name: "G1", Buckets: []QuotaBucket{{RemainingFraction: 0.884}, {RemainingFraction: 0.932}}},
			{Name: "G2", Buckets: []QuotaBucket{{RemainingFraction: 1.0}}},
			{Name: "G3", Buckets: []QuotaBucket{{RemainingFraction: 0.696}}},
		},
	}
	if got := LowestFraction(data); math.Abs(got-0.696) > 1e-9 {
		t.Fatalf("expected 0.696, got %v", got)
	}
}

func TestFormatCountdown(t *testing.T) {
	now := time.Date(2026, 11, 17, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		resetISO   string
		wantPrefix string
		exact      string
	}{
		{
			name:       "days and hours",
			resetISO:   now.Add(3*24*time.Hour + 6*time.Hour).Format(time.RFC3339),
			wantPrefix: "Reseta em 3d 06h (",
		},
		{
			name:       "hours and minutes",
			resetISO:   now.Add(4*time.Hour + 22*time.Minute).Format(time.RFC3339),
			wantPrefix: "Reseta em 4h 22m (",
		},
		{
			name:       "minutes only",
			resetISO:   now.Add(15 * time.Minute).Format(time.RFC3339),
			wantPrefix: "Reseta em 15m (",
		},
		{
			name:       "under one minute",
			resetISO:   now.Add(30 * time.Second).Format(time.RFC3339),
			wantPrefix: "Reseta em <1m (",
		},
		{
			name:     "expired",
			resetISO: now.Add(-1 * time.Minute).Format(time.RFC3339),
			exact:    "Resetando agora",
		},
		{
			name:     "invalid timestamp",
			resetISO: "not-a-timestamp",
			exact:    "Horário de reset indisponível",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatCountdown(tc.resetISO, now)
			if tc.exact != "" {
				if got != tc.exact {
					t.Fatalf("expected %q, got %q", tc.exact, got)
				}
				return
			}
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("expected prefix %q, got %q", tc.wantPrefix, got)
			}
		})
	}
}

func TestFriendlyBucketNameAndSubtitle(t *testing.T) {
	bucketCases := []struct {
		bucket QuotaBucket
		want   string
	}{
		{QuotaBucket{ID: "gemini-argon-wtu-weekly", Window: "weekly"}, "Limite Semanal Argon"},
		{QuotaBucket{ID: "gemini-5h", Window: "5h"}, "Limite de 5 Horas"},
		{QuotaBucket{ID: "gemini-weekly", Window: "weekly"}, "Limite Semanal"},
		{QuotaBucket{ID: "custom", Name: "Custom Bucket", Window: "monthly"}, "Custom Bucket"},
	}

	for _, tc := range bucketCases {
		if got := FriendlyBucketName(tc.bucket); got != tc.want {
			t.Fatalf("FriendlyBucketName(%+v) = %q, want %q", tc.bucket, got, tc.want)
		}
	}

	subCases := []struct {
		group QuotaGroup
		want  string
	}{
		{QuotaGroup{Description: "Models within this group: Gemini 4 Hydrogen, Gemini 3.8 Flash"}, "Gemini 4 Hydrogen, Gemini 3.8 Flash"},
		{QuotaGroup{Description: "For paid Pro plans, Argon quota is separate."}, "Cota semanal dedicada (Plano Pro)"},
		{QuotaGroup{Description: "  Other info  "}, "Other info"},
	}

	for _, tc := range subCases {
		if got := ShortGroupSubtitle(tc.group); got != tc.want {
			t.Fatalf("ShortGroupSubtitle(%q) = %q, want %q", tc.group.Description, got, tc.want)
		}
	}
}
