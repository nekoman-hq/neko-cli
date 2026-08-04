package doctor

import (
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// integrationDoctorContextValidatorCommand is the canonical Neko command a
// consumer workflow must run to validate its dispatched release context.
const integrationDoctorContextValidatorCommand = "neko release ci-validate-context"

// integrationDoctorShellCommand normalizes one workflow shell command so
// contract recognition stays independent from line continuations and
// indentation. Backslash continuations are joined and every whitespace run
// collapses to a single space.
func integrationDoctorShellCommand(run string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(run, "\\\n", " ")), " ")
}

// integrationDoctorStepEvidence returns the normalized shell command of one
// effective step together with its effective environment values. A composite
// action binds most of its values through `env`, so recognition must accept a
// value whether it is written inline or supplied by the invoking step.
func integrationDoctorStepEvidence(step integrationDoctorWorkflowStep) string {
	evidence := make([]string, 0, 4)
	evidence = append(evidence, integrationDoctorShellCommand(step.run))
	environment := workflowMappingValue(step.node, "env")
	if environment != nil && environment.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(environment.Content); index += 2 {
			evidence = append(evidence, workflowScalar(environment.Content[index+1]))
		}
	}
	return strings.Join(evidence, " ")
}

// integrationDoctorGoBuildPackages maps each `go build -o <output>` target to
// the Go package the command builds. The package is the command's trailing
// positional argument.
func integrationDoctorGoBuildPackages(run string) map[string]string {
	packages := make(map[string]string)
	for _, command := range integrationDoctorShellCommands(run) {
		fields := strings.Fields(command)
		if len(fields) < 4 || fields[0] != "go" || fields[1] != "build" {
			continue
		}
		buildPackage := integrationDoctorUnquote(fields[len(fields)-1])
		output := ""
		for index := 2; index+1 < len(fields); index++ {
			if integrationDoctorUnquote(fields[index]) == "-o" {
				output = integrationDoctorUnquote(fields[index+1])
			}
		}
		if output == "" || buildPackage == output || strings.HasPrefix(buildPackage, "-") {
			continue
		}
		packages[output] = path.Clean(buildPackage)
	}
	return packages
}

// integrationDoctorGoBuildOutputs lists every executable a command builds.
func integrationDoctorGoBuildOutputs(run string) []string {
	packages := integrationDoctorGoBuildPackages(run)
	outputs := make([]string, 0, len(packages))
	for output := range packages {
		outputs = append(outputs, output)
	}
	sort.Strings(outputs)
	return outputs
}

// integrationDoctorAppendsDirectoryToPath reports whether the command exports
// directory to the later workflow PATH.
func integrationDoctorAppendsDirectoryToPath(run, directory string) bool {
	if directory == "" || directory == "." {
		return false
	}
	for _, command := range integrationDoctorShellCommands(run) {
		if strings.Contains(command, "GITHUB_PATH") &&
			strings.Contains(integrationDoctorUnquoteAll(command), integrationDoctorUnquoteAll(directory)) {
			return true
		}
	}
	return false
}

// integrationDoctorShellVariableName returns the shell variable a path is
// rooted in, such as NEKO_BIN_DIR for "$NEKO_BIN_DIR/neko".
func integrationDoctorShellVariableName(value string) string {
	rooted := integrationDoctorUnquote(value)
	if !strings.HasPrefix(rooted, "$") {
		return ""
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(rooted, "${"), "$"), "/")
	return strings.TrimSuffix(name, "}")
}

// integrationDoctorShellCommands splits one shell script into its logical
// commands, joining backslash continuations first.
func integrationDoctorShellCommands(run string) []string {
	joined := strings.ReplaceAll(run, "\\\n", " ")
	commands := make([]string, 0, 8)
	for _, line := range strings.Split(joined, "\n") {
		for _, command := range strings.FieldsFunc(line, func(separator rune) bool {
			return separator == ';' || separator == '|'
		}) {
			commands = append(commands, strings.TrimSpace(command))
		}
	}
	return commands
}

func integrationDoctorUnquote(value string) string {
	return strings.Trim(value, `"'`)
}

func integrationDoctorUnquoteAll(value string) string {
	return strings.NewReplacer(`"`, "", `'`, "").Replace(value)
}
