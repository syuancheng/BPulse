package bpentry

import "testing"

func TestInterpretTextChineseAndArabicNumerals(t *testing.T) {
	result, err := InterpretText("高压是一百三十二，低压为84，心率约七十")
	if err != nil {
		t.Fatalf("InterpretText() error = %v", err)
	}
	if *result.Candidate.Systolic.Value != 132 || *result.Candidate.Diastolic.Value != 84 || *result.Candidate.Pulse.Value != 70 {
		t.Fatalf("candidate = %#v", result.Candidate)
	}
	if !result.NeedsConfirmation {
		t.Fatal("recognition result must require confirmation")
	}
}

func TestInterpretTextAbbreviatedHundreds(t *testing.T) {
	result, err := InterpretText("高压一百二，低压八十")
	if err != nil {
		t.Fatalf("InterpretText() error = %v", err)
	}
	if *result.Candidate.Systolic.Value != 120 || *result.Candidate.Diastolic.Value != 80 {
		t.Fatalf("candidate = %#v", result.Candidate)
	}
}

func TestInterpretTextSynonymsAndConflictBlank(t *testing.T) {
	result, err := InterpretText("收缩压130，上压135，舒张压80，脉搏68")
	if err != nil {
		t.Fatalf("InterpretText() error = %v", err)
	}
	if result.Candidate.Systolic.Value != nil || result.Candidate.Systolic.Reason != "conflict" {
		t.Fatalf("systolic = %#v, want conflict blank", result.Candidate.Systolic)
	}
	if *result.Candidate.Diastolic.Value != 80 || *result.Candidate.Pulse.Value != 68 {
		t.Fatalf("candidate = %#v", result.Candidate)
	}
}

func TestInterpretTextMissingLabelsRemainBlank(t *testing.T) {
	result, err := InterpretText("132 84 70")
	if err != nil {
		t.Fatalf("InterpretText() error = %v", err)
	}
	if result.Candidate.Systolic.Value != nil || result.Candidate.Diastolic.Value != nil || result.Candidate.Pulse.Value != nil {
		t.Fatalf("unlabeled values should stay blank: %#v", result.Candidate)
	}
}
