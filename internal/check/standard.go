package check

// The standard check -- phasecheck then phasebuild and
// nothing else -- and the eleven phases whose whole Body it is: every one of
// them rests its claim on the sweep and the symbol count, and says so in the
// header of its shell.
func init() {
	for _, n := range []string{"whim1", "whim2", "whim4", "whim5", "whim6", "whim7",
		"whim10", "whim13", "whim14", "whim15", "whim48"} {
		Register(n, stdWhim(n))
	}
}
