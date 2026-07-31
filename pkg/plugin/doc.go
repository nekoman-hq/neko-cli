// Package plugin defines the public plugin process protocol, manifests,
// installation APIs, and compatibility surfaces used by plugin authors.
// Responses explicitly request a portable Core process exit with SetExitCode;
// an omitted exit remains a deprecated implicit-success compatibility case for
// installed legacy plugins.
package plugin
