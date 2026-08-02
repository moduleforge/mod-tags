package service

import (
	"context"
	"errors"
	"testing"
)

func TestTagPurposePolicyService_Get_UnsetPurpose(t *testing.T) {
	tagQ := newMockTagQuerier()
	svc := &TagPurposePolicyService{}

	got, err := svc.Get(context.Background(), tagQ, "priority")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := TagPurposePolicy{Purpose: "priority", OneOfDomain: false}
	if got != want {
		t.Errorf("Get: got %+v, want %+v", got, want)
	}
}

func TestTagPurposePolicyService_Get_MissingPurpose(t *testing.T) {
	tagQ := newMockTagQuerier()
	svc := &TagPurposePolicyService{}

	_, err := svc.Get(context.Background(), tagQ, "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err: got %v, want ErrInvalidInput", err)
	}
}

func TestTagPurposePolicyService_Get_TrimsPurpose(t *testing.T) {
	tagQ := newMockTagQuerier()
	tagQ.seedPurposePolicy("priority", true)
	svc := &TagPurposePolicyService{}

	got, err := svc.Get(context.Background(), tagQ, "  priority  ")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.OneOfDomain {
		t.Errorf("Get.OneOfDomain: got false, want true")
	}
}

func TestTagPurposePolicyService_Upsert_MissingPurpose(t *testing.T) {
	tagQ := newMockTagQuerier()
	svc := &TagPurposePolicyService{}

	_, err := svc.Upsert(context.Background(), tagQ, "", true)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err: got %v, want ErrInvalidInput", err)
	}
}

func TestTagPurposePolicyService_UpsertThenGet_RoundTrips(t *testing.T) {
	tagQ := newMockTagQuerier()
	svc := &TagPurposePolicyService{}

	upserted, err := svc.Upsert(context.Background(), tagQ, "priority", true)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	want := TagPurposePolicy{Purpose: "priority", OneOfDomain: true}
	if upserted != want {
		t.Errorf("Upsert: got %+v, want %+v", upserted, want)
	}

	got, err := svc.Get(context.Background(), tagQ, "priority")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("Get after Upsert: got %+v, want %+v", got, want)
	}
}

func TestTagPurposePolicyService_Upsert_UpdatesInPlace(t *testing.T) {
	tagQ := newMockTagQuerier()
	svc := &TagPurposePolicyService{}

	if _, err := svc.Upsert(context.Background(), tagQ, "priority", true); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	second, err := svc.Upsert(context.Background(), tagQ, "priority", false)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if second.OneOfDomain {
		t.Errorf("second.OneOfDomain: got true, want false")
	}

	if len(tagQ.policies) != 1 {
		t.Fatalf("len(tagQ.policies): got %d, want 1 (update in place, not a duplicate entry)", len(tagQ.policies))
	}

	got, err := svc.Get(context.Background(), tagQ, "priority")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.OneOfDomain {
		t.Errorf("Get after second Upsert: got OneOfDomain=true, want false")
	}
}
