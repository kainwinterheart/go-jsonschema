package tests_test

import (
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/benbjohnson/immutable"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	test "github.com/kainwinterheart/go-jsonschema/tests/data/core/testSchema"
	"dt"
)

func TestRoundTripTestSchema(t *testing.T) {
	t.Parallel()

	// ---- Build immutable collections ----

	pdAllChangesRaw := immutable.NewMapOf[string](nil, map[string]string{"key": "val"})
	dsAllChangesRaw := immutable.NewMapOf[string](nil, map[string]string{"change": "updated"})
	coderOutputs := immutable.NewList(dt.Coder{})
	// Shared populated InvestigatorFindings for findingsList and investigatorFindings in PayloadData
	payloadFinding := (&dt.InvestigatorFindingsBuilder{}).
		WithConclusions(immutable.NewList[string]("Conclusion: Data integrity verified")).
		WithConfidenceLevel(dt.InvestigatorFindingsconfidencelevelHigh).
		WithSupportingEvidence(immutable.NewList(
			*((&dt.InvestigatorFindingssupportingevidenceElemBuilder{}).
				WithEvidenceDescription("Database checksums match").
				WithEvidenceType("code_snippet").
				WithSourceReference("db-checksum-v3").
				Build()),
		)).
		WithUnansweredQuestions(immutable.NewList[string]()).
		WithWorkstreamObjective("Validate data integrity").
		Build()
	findingsList := immutable.NewList[dt.InvestigatorFindings](*payloadFinding)
	coderOutputsDS := immutable.NewList(dt.Coder{})
	codeSummaries := immutable.NewList[string]("summary1", "summary2")
	speculativeExpansions := immutable.NewList[string]("exp1", "exp2")
	workstreams := immutable.NewList(dt.InvestigatorPlanworkstreamsElem{})

	// ---- Build DomainState (used in nested domains map) ----
	ds := new(test.DomainStateBuilder).
		WithAllChanges(toDomainStateAllChanges(dsAllChangesRaw)).
		WithArchitecture(&dt.Arch{}).
		WithPlan(&dt.Plan{}).
		WithCodeOutputs(coderOutputsDS).
		WithCodeSummaries(codeSummaries).
		WithCodeSummary("code-summary").
		WithFinalFeedback(&dt.ArchFinal{}).
		WithFinalReviewPassed(true).
		WithMergedSummaries("merged").
		WithPlan(&dt.Plan{}).
		WithPmFilepath("/some/path").
		WithTechLeadReviewIter(3).
		WithTechLeadReviewResult(&dt.TechLeadFinal{}).
		WithTechLeadRevisionCycle(false).
		WithWrappedCoderTask("wrapped-coder-task").
		WithWrappedTask("wrapped-task").
		Build()

	// ---- Build PayloadData ----
	payloadData := new(test.PayloadDataBuilder).
		WithAllChanges(toPayloadDataAllChanges(pdAllChangesRaw)).
		WithArchFinal(&dt.ArchFinal{}).
		WithArchReview(&dt.ArchReview{}).
		WithCodeReview(&dt.CodeReview{}).
		WithCoderOutputs(coderOutputs).
		WithConsistencyReview(&dt.SynthesisConsistencyReview{}).
		WithDecompositionReview(&dt.SystemDecompositionReview{}).
		WithDocSuffix("doc-suffix").
		WithFactReview(&dt.FactCheckingReview{}).
		WithFindingsList(findingsList).
		WithGapAnalysisReview(&dt.GapAnalysisReview{}).
		WithHasTechLeadFinalReview(true).
		WithInitialPromptContext("initial-context").
		WithInvestigationPlanQualityReview(&dt.InvestigationPlanQualityReview{}).
		WithInvestigationReport(&dt.InvestigationReport{}).
		WithInvestigatorFindings(payloadFinding).
		WithIterCount(42).
		WithPlanReview(&dt.PlanReview{}).
		WithRevisionInvPrefix("rev-inv-prefix").
		WithRevisionPrompt("revision-prompt").
		WithTechLeadFinalReview(&dt.TechLeadFinal{}).
		WithTlIter(7).
		WithWorkstreamElem(&dt.InvestigatorPlanworkstreamsElem{}).
		Build()

	// ---- Build populated InvestigatorFindings for completedWorkstreams ----
	findingW1 := (&dt.InvestigatorFindingsBuilder{}).
		WithConclusions(immutable.NewList[string]("Conclusion: System stable", "Conclusion: No regressions")).
		WithConfidenceLevel(dt.InvestigatorFindingsconfidencelevelHigh).
		WithSupportingEvidence(immutable.NewList(
			*((&dt.InvestigatorFindingssupportingevidenceElemBuilder{}).
				WithEvidenceDescription("All unit tests passing").
				WithEvidenceType("log_entry").
				WithSourceReference("build-log-42").
				Build()),
			*((&dt.InvestigatorFindingssupportingevidenceElemBuilder{}).
				WithEvidenceDescription("Coverage above 90%").
				WithEvidenceType("metric_value").
				WithSourceReference("coverage-report").
				Build()),
		)).
		WithUnansweredQuestions(immutable.NewList[string]("Should we migrate to Go generics?")).
		WithWorkstreamObjective("Verify system stability").
		Build()
	findingW2 := (&dt.InvestigatorFindingsBuilder{}).
		WithConclusions(immutable.NewList[string]("Conclusion: Performance degraded")).
		WithConfidenceLevel(dt.InvestigatorFindingsconfidencelevelMedium).
		WithSupportingEvidence(immutable.NewList[dt.InvestigatorFindingssupportingevidenceElem]()).
		WithUnansweredQuestions(immutable.NewList[string]("Root cause unknown", "Impact scope unclear")).
		WithWorkstreamObjective("Investigate performance regression").
		Build()

	// Domains map with a fully-populated DomainState value
	domainsMapRaw := immutable.NewMapOf[string](nil, map[string]test.DomainState{
		"d1": *ds,
	})

	// ---- Build WorkflowState ----
	completedWorkstreamsRaw := immutable.NewMapOf[string](nil, map[string]dt.InvestigatorFindings{
		"w1": *findingW1,
		"w2": *findingW2,
	})
	workflowState := new(test.WorkflowStateBuilder).
		WithCompletedWorkstreams(toWorkflowStateCompletedWorkstreams(completedWorkstreamsRaw)).
		WithDecompositionResult(&dt.SystemDecomposition{}).
		WithDomainCurrentStage("design").
		WithDomainIterationIndex(1).
		WithDomainIterationTotal(5).
		WithDomains(toWorkflowStateDomains(domainsMapRaw)).
		WithFinalInvestigationTask("finalize").
		WithInvestigationPlan(&dt.InvestigatorPlan{}).
		WithInvestigationResults(&dt.InvestigationReport{}).
		WithOut("done").
		WithPmFilepath("/pm/path").
		WithRephrasedTask(&dt.PmSynthesizer{}).
		WithSpeculativeExpansions(speculativeExpansions).
		WithSubdir("sub").
		WithTask("main-task").
		WithTechLeadDocSuffix("-TL").
		WithWorkstreams(workstreams).
		Build()

	// ---- Build TestSchema (top-level) ----
	example := new(test.TestSchemaBuilder).
		WithCodeReviewIteration(2).
		WithDomainID("domain-123").
		WithDomainIndex(42).
		WithIteration(1).
		WithPayload(payloadData).
		WithReviewIteration(3).
		WithState(workflowState).
		WithStepType(test.TestSchemasteptypeArchitecture).
		WithWorkstreamID("ws-456").
		WithWorkstreamIndex(7).
		Build()

	// ---- Serialize ----
	serialized, err := json.Marshal(example)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	// ---- Deserialize ----
	var deserialized test.TestSchema
	if err := json.Unmarshal(serialized, &deserialized); err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	// ---- Compare ALL fields ----
	before := convertToComparable(example)
	after := convertToComparable(&deserialized)
	if diff := cmp.Diff(before, after, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(
		dt.InvestigatorFindings{},
		dt.InvestigatorFindingssupportingevidenceElem{},
	)); diff != "" {
		t.Fatalf("field mismatch after round-trip:\n%s", diff)
	}
}

// === Comparable types (all exported fields) ===

type comparableTestSchema struct {
	CodeReviewIteration int
	DomainID            string
	DomainIndex         int
	Iteration           int
	Payload             *comparablePayloadData
	ReviewIteration     int
	State               *comparableWorkflowState
	StepType            string
	WorkstreamID        string
	WorkstreamIndex     int
}

type comparablePayloadData struct {
	AllChanges                     map[string]string
	ArchFinal                      *dt.ArchFinal
	ArchReview                     *dt.ArchReview
	CodeReview                     *dt.CodeReview
	CoderOutputs                   []dt.Coder
	ConsistencyReview              *dt.SynthesisConsistencyReview
	DecompositionReview            *dt.SystemDecompositionReview
	DocSuffix                      string
	FactReview                     *dt.FactCheckingReview
	FindingsList                   []dt.InvestigatorFindings
	GapAnalysisReview              *dt.GapAnalysisReview
	HasTechLeadFinalReview         bool
	InitialPromptContext           string
	InvestigationPlanQualityReview *dt.InvestigationPlanQualityReview
	InvestigationReport            *dt.InvestigationReport
	InvestigatorFindings           *dt.InvestigatorFindings
	IterCount                      int
	PlanReview                     *dt.PlanReview
	RevisionInvPrefix              string
	RevisionPrompt                 string
	TechLeadFinalReview            *dt.TechLeadFinal
	TlIter                         int
	WorkstreamElem                 *dt.InvestigatorPlanworkstreamsElem
}

type comparableDomainState struct {
	AllChanges              map[string]string
	Architecture            *dt.Arch
	CodeOutputs             []dt.Coder
	CodeSummaries           []string
	CodeSummary             string
	FinalFeedback           *dt.ArchFinal
	FinalReviewPassed       bool
	MergedSummaries         string
	Plan                    *dt.Plan
	PmFilepath              string
	TechLeadReviewIter      int
	TechLeadReviewResult    *dt.TechLeadFinal
	TechLeadRevisionCycle   bool
	WrappedCoderTask        string
	WrappedTask             string
}

type comparableInvestigatorFindingssupportingevidenceElem struct {
	EvidenceDescription string
	EvidenceType        string
	SourceReference     string
}

type comparableInvestigatorFindings struct {
	Conclusions         []string
	ConfidenceLevel     string
	SupportingEvidence  []comparableInvestigatorFindingssupportingevidenceElem
	UnansweredQuestions []string
	WorkstreamObjective string
}

type comparableWorkflowState struct {
	Choices                 string
	CompletedWorkstreams    map[string]comparableInvestigatorFindings
	DecompositionResult     *dt.SystemDecomposition
	DomainCurrentStage      string
	DomainIterationIndex    int
	DomainIterationTotal    int
	Domains                 map[string]comparableDomainState
	FinalInvestigationTask  string
	InvestigationPlan       *dt.InvestigatorPlan
	InvestigationResults    *dt.InvestigationReport
	Out                     string
	PmFilepath              string
	RephrasedTask           *dt.PmSynthesizer
	SpeculativeExpansions   []string
	Subdir                  string
	Task                    string
	TechLeadDocSuffix       string
	Workstreams             []dt.InvestigatorPlanworkstreamsElem
}

// === Unsafe type casting helpers ===
// The generated code uses distinct named type aliases for immutable.Map, but they
// all share the same underlying type (*immutable.Map[K,V]). We use unsafe pointer
// conversion to bridge between them.

func toDomainStateAllChanges(m *immutable.Map[string, string]) test.DomainStateallchanges {
	return *(*test.DomainStateallchanges)(unsafe.Pointer(m))
}

func toPayloadDataAllChanges(m *immutable.Map[string, string]) test.PayloadDataallchanges {
	return *(*test.PayloadDataallchanges)(unsafe.Pointer(m))
}

func toWorkflowStateCompletedWorkstreams(m *immutable.Map[string, dt.InvestigatorFindings]) test.WorkflowStatecompletedworkstreams {
	return *(*test.WorkflowStatecompletedworkstreams)(unsafe.Pointer(m))
}

func toWorkflowStateDomains(m *immutable.Map[string, test.DomainState]) test.WorkflowStatedomains {
	return *(*test.WorkflowStatedomains)(unsafe.Pointer(m))
}

// === Conversion helpers ===

func convertToComparable(ts *test.TestSchema) *comparableTestSchema {
	if ts == nil {
		return nil
	}
	return &comparableTestSchema{
		CodeReviewIteration: ts.CodeReviewIteration(),
		DomainID:            ts.DomainID(),
		DomainIndex:         ts.DomainIndex(),
		Iteration:           ts.Iteration(),
		Payload:             convertPayloadData(ts.Payload()),
		ReviewIteration:     ts.ReviewIteration(),
		State:               convertWorkflowState(ts.State()),
		StepType:            string(ts.StepType()),
		WorkstreamID:        ts.WorkstreamID(),
		WorkstreamIndex:     ts.WorkstreamIndex(),
	}
}

func convertPayloadData(pd *test.PayloadData) *comparablePayloadData {
	if pd == nil {
		return nil
	}
	items := pd.AllChanges().Items()
	allChanges := make(map[string]string, len(items))
	for _, it := range items {
		allChanges[it.Key] = it.Value
	}
	return &comparablePayloadData{
		AllChanges:                     allChanges,
		ArchFinal:                      pd.ArchFinal(),
		ArchReview:                     pd.ArchReview(),
		CodeReview:                     pd.CodeReview(),
		CoderOutputs:                   toList(dt.Coder{}, pd.CoderOutputs()),
		ConsistencyReview:              pd.ConsistencyReview(),
		DecompositionReview:            pd.DecompositionReview(),
		DocSuffix:                      pd.DocSuffix(),
		FactReview:                     pd.FactReview(),
		FindingsList:                   toList(dt.InvestigatorFindings{}, pd.FindingsList()),
		GapAnalysisReview:              pd.GapAnalysisReview(),
		HasTechLeadFinalReview:         pd.HasTechLeadFinalReview(),
		InitialPromptContext:           pd.InitialPromptContext(),
		InvestigationPlanQualityReview: pd.InvestigationPlanQualityReview(),
		InvestigationReport:            pd.InvestigationReport(),
		InvestigatorFindings:           pd.InvestigatorFindings(),
		IterCount:                      pd.IterCount(),
		PlanReview:                     pd.PlanReview(),
		RevisionInvPrefix:              pd.RevisionInvPrefix(),
		RevisionPrompt:                 pd.RevisionPrompt(),
		TechLeadFinalReview:            pd.TechLeadFinalReview(),
		TlIter:                         pd.TlIter(),
		WorkstreamElem:                 pd.WorkstreamElem(),
	}
}

func convertDomainState(ds test.DomainState) comparableDomainState {
	items := ds.AllChanges().Items()
	allChanges := make(map[string]string, len(items))
	for _, it := range items {
		allChanges[it.Key] = it.Value
	}
	return comparableDomainState{
		AllChanges:            allChanges,
		Architecture:          ds.Architecture(),
		CodeOutputs:           toList(dt.Coder{}, ds.CodeOutputs()),
		CodeSummaries:         toList("", ds.CodeSummaries()),
		CodeSummary:           ds.CodeSummary(),
		FinalFeedback:         ds.FinalFeedback(),
		FinalReviewPassed:     ds.FinalReviewPassed(),
		MergedSummaries:       ds.MergedSummaries(),
		Plan:                  ds.Plan(),
		PmFilepath:            ds.PmFilepath(),
		TechLeadReviewIter:    ds.TechLeadReviewIter(),
		TechLeadReviewResult:  ds.TechLeadReviewResult(),
		TechLeadRevisionCycle: ds.TechLeadRevisionCycle(),
		WrappedCoderTask:      ds.WrappedCoderTask(),
		WrappedTask:           ds.WrappedTask(),
	}
}

func convertInvestigatorFindings(f dt.InvestigatorFindings) comparableInvestigatorFindings {
	se := toList(dt.InvestigatorFindingssupportingevidenceElem{}, f.SupportingEvidence())
	seComp := make([]comparableInvestigatorFindingssupportingevidenceElem, len(se))
	for i, elem := range se {
		seComp[i] = comparableInvestigatorFindingssupportingevidenceElem{
			EvidenceDescription: elem.EvidenceDescription(),
			EvidenceType:        elem.EvidenceType(),
			SourceReference:     elem.SourceReference(),
		}
	}
	return comparableInvestigatorFindings{
		Conclusions:         toList("", f.Conclusions()),
		ConfidenceLevel:     string(f.ConfidenceLevel()),
		SupportingEvidence:  seComp,
		UnansweredQuestions: toList("", f.UnansweredQuestions()),
		WorkstreamObjective: f.WorkstreamObjective(),
	}
}

func convertWorkflowState(ws *test.WorkflowState) *comparableWorkflowState {
	if ws == nil {
		return nil
	}
	completedItems := ws.CompletedWorkstreams().Items()
	completedWorkstreams := make(map[string]comparableInvestigatorFindings, len(completedItems))
	for _, it := range completedItems {
		completedWorkstreams[it.Key] = convertInvestigatorFindings(it.Value)
	}
	domainsItems := ws.Domains().Items()
	domains := make(map[string]comparableDomainState, len(domainsItems))
	for _, it := range domainsItems {
		domains[it.Key] = convertDomainState(it.Value)
	}
	return &comparableWorkflowState{
		Choices:                 ws.Choices(),
		CompletedWorkstreams:    completedWorkstreams,
		DecompositionResult:     ws.DecompositionResult(),
		DomainCurrentStage:      ws.DomainCurrentStage(),
		DomainIterationIndex:    ws.DomainIterationIndex(),
		DomainIterationTotal:    ws.DomainIterationTotal(),
		Domains:                 domains,
		FinalInvestigationTask:  ws.FinalInvestigationTask(),
		InvestigationPlan:       ws.InvestigationPlan(),
		InvestigationResults:    ws.InvestigationResults(),
		Out:                     ws.Out(),
		PmFilepath:              ws.PmFilepath(),
		RephrasedTask:           ws.RephrasedTask(),
		SpeculativeExpansions:   toList("", ws.SpeculativeExpansions()),
		Subdir:                  ws.Subdir(),
		Task:                    ws.Task(),
		TechLeadDocSuffix:       ws.TechLeadDocSuffix(),
		Workstreams:             toList(dt.InvestigatorPlanworkstreamsElem{}, ws.Workstreams()),
	}
}

func toList[T comparable](zero T, l *immutable.List[T]) []T {
	if l == nil || l.Len() == 0 {
		return nil
	}
	s := make([]T, l.Len())
	for i := 0; i < l.Len(); i++ {
		s[i] = l.Get(i)
	}
	return s
}
