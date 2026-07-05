package check

type Reporter interface {
	Start(result Result)
	StepStart(name string)
	StepDone(step Step)
	Finish(result Result)
}
