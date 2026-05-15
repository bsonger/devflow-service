package domain

import (
	"reflect"
	"testing"
)

func TestWorkloadConfigContractUsesConstrainedTypes(t *testing.T) {
	cfgType := reflect.TypeOf(WorkloadConfig{})
	resourcesField, ok := cfgType.FieldByName("Resources")
	if !ok {
		t.Fatal("WorkloadConfig missing Resources field")
	}
	if got := resourcesField.Type; got != reflect.TypeOf(WorkloadResourceRequirements{}) {
		t.Fatalf("WorkloadConfig.Resources type = %v, want %v", got, reflect.TypeOf(WorkloadResourceRequirements{}))
	}
	probesField, ok := cfgType.FieldByName("Probes")
	if !ok {
		t.Fatal("WorkloadConfig missing Probes field")
	}
	if got := probesField.Type; got != reflect.TypeOf(WorkloadProbes{}) {
		t.Fatalf("WorkloadConfig.Probes type = %v, want %v", got, reflect.TypeOf(WorkloadProbes{}))
	}
	emptyDirsField, ok := cfgType.FieldByName("EmptyDirs")
	if !ok {
		t.Fatal("WorkloadConfig missing EmptyDirs field")
	}
	if got := emptyDirsField.Type; got != reflect.TypeOf([]WorkloadEmptyDir{}) {
		t.Fatalf("WorkloadConfig.EmptyDirs type = %v, want %v", got, reflect.TypeOf([]WorkloadEmptyDir{}))
	}

	inputType := reflect.TypeOf(WorkloadConfigInput{})
	inputResourcesField, ok := inputType.FieldByName("Resources")
	if !ok {
		t.Fatal("WorkloadConfigInput missing Resources field")
	}
	if got := inputResourcesField.Type; got != reflect.TypeOf(WorkloadResourceRequirements{}) {
		t.Fatalf("WorkloadConfigInput.Resources type = %v, want %v", got, reflect.TypeOf(WorkloadResourceRequirements{}))
	}
	inputProbesField, ok := inputType.FieldByName("Probes")
	if !ok {
		t.Fatal("WorkloadConfigInput missing Probes field")
	}
	if got := inputProbesField.Type; got != reflect.TypeOf(WorkloadProbes{}) {
		t.Fatalf("WorkloadConfigInput.Probes type = %v, want %v", got, reflect.TypeOf(WorkloadProbes{}))
	}
	inputEmptyDirsField, ok := inputType.FieldByName("EmptyDirs")
	if !ok {
		t.Fatal("WorkloadConfigInput missing EmptyDirs field")
	}
	if got := inputEmptyDirsField.Type; got != reflect.TypeOf([]WorkloadEmptyDir{}) {
		t.Fatalf("WorkloadConfigInput.EmptyDirs type = %v, want %v", got, reflect.TypeOf([]WorkloadEmptyDir{}))
	}
}

func TestWorkloadConfigContractNoLongerExposesWideMaps(t *testing.T) {
	for _, tc := range []struct {
		name   string
		typeOf reflect.Type
	}{
		{name: "WorkloadConfig", typeOf: reflect.TypeOf(WorkloadConfig{})},
		{name: "WorkloadConfigInput", typeOf: reflect.TypeOf(WorkloadConfigInput{})},
	} {
		for _, fieldName := range []string{"Resources", "Probes"} {
			field, ok := tc.typeOf.FieldByName(fieldName)
			if !ok {
				t.Fatalf("%s missing %s field", tc.name, fieldName)
			}
			if field.Type.Kind() == reflect.Map {
				t.Fatalf("%s.%s must not be a wide map anymore; got %v", tc.name, fieldName, field.Type)
			}
		}
	}
}

func TestWorkloadSizeClassResourcesCoversCanonicalClasses(t *testing.T) {
	want := map[WorkloadSizeClass]WorkloadResourceRequirements{
		WorkloadSizeClassSmall: {
			SizeClass: WorkloadSizeClassSmall,
			Requests:  WorkloadResourceList{CPU: "100m", Memory: "128Mi"},
			Limits:    WorkloadResourceList{CPU: "500m", Memory: "512Mi"},
		},
		WorkloadSizeClassMedium: {
			SizeClass: WorkloadSizeClassMedium,
			Requests:  WorkloadResourceList{CPU: "250m", Memory: "256Mi"},
			Limits:    WorkloadResourceList{CPU: "1", Memory: "1Gi"},
		},
		WorkloadSizeClassLarge: {
			SizeClass: WorkloadSizeClassLarge,
			Requests:  WorkloadResourceList{CPU: "500m", Memory: "512Mi"},
			Limits:    WorkloadResourceList{CPU: "2", Memory: "2Gi"},
		},
		WorkloadSizeClassXLarge: {
			SizeClass: WorkloadSizeClassXLarge,
			Requests:  WorkloadResourceList{CPU: "1", Memory: "1Gi"},
			Limits:    WorkloadResourceList{CPU: "4", Memory: "4Gi"},
		},
	}

	if !reflect.DeepEqual(WorkloadSizeClassResources, want) {
		t.Fatalf("WorkloadSizeClassResources = %#v, want %#v", WorkloadSizeClassResources, want)
	}
}
