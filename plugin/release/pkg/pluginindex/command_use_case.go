package pluginindex

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type pluginIndexCommandResult struct {
	Index      *Index
	Repository string
	OutputPath string
	RawOutput  string
	Mode       pluginIndexCommandMode
	Target     pluginIndexOutputTarget
	Plugins    int
	Pretty     bool
}

type pluginIndexCommandRunner interface {
	Run(context.Context, pluginIndexCommandRequest) (pluginIndexCommandResult, error)
}

type pluginIndexQuerier interface {
	Query(context.Context, GenerateOptions) (*Index, error)
}

type pluginIndexOutputBuilder interface {
	Build(*Index, WriteOptions) ([]byte, error)
}

type pluginIndexOutputPersister interface {
	Persist(string, []byte) error
}

type generatePluginIndexUseCase struct {
	query          pluginIndexQuerier
	builder        pluginIndexOutputBuilder
	persister      pluginIndexOutputPersister
	repositoryRoot string
}

func newGeneratePluginIndexUseCase(
	query pluginIndexQuerier,
	builder pluginIndexOutputBuilder,
	persister pluginIndexOutputPersister,
) generatePluginIndexUseCase {
	return newGeneratePluginIndexUseCaseAt(".", query, builder, persister)
}

func newGeneratePluginIndexUseCaseAt(
	root string,
	query pluginIndexQuerier,
	builder pluginIndexOutputBuilder,
	persister pluginIndexOutputPersister,
) generatePluginIndexUseCase {
	return generatePluginIndexUseCase{repositoryRoot: root, query: query, builder: builder, persister: persister}
}

func (useCase generatePluginIndexUseCase) Run(ctx context.Context, request pluginIndexCommandRequest) (pluginIndexCommandResult, error) {
	logPluginIndexStart(request.Mode)
	index, err := useCase.query.Query(ctx, GenerateOptions{Root: useCase.repositoryRoot, Repository: request.Repository})
	if err != nil {
		logPluginIndexRefusal(request.Mode, "source validation")
		return pluginIndexCommandResult{}, err
	}
	logPluginIndexValidation(request.Mode)
	result := pluginIndexCommandResult{
		Repository: index.Repository,
		OutputPath: request.OutputPath,
		Mode:       request.Mode,
		Plugins:    len(index.Plugins),
		Index:      index,
		Pretty:     request.Pretty,
	}
	if request.Mode == pluginIndexCheckMode {
		log.PluginV(log.Exec, "Plugin Index validation completed")
		return result, nil
	}

	output, err := useCase.builder.Build(index, WriteOptions{Pretty: request.Pretty})
	if err != nil {
		logPluginIndexRefusal(request.Mode, "artifact construction")
		return pluginIndexCommandResult{}, err
	}
	if request.Mode == pluginIndexRenderMode {
		result.RawOutput = string(output)
		return result, nil
	}
	log.PluginV(log.Config, "Resolving Plugin Index output target")
	target, err := resolvePluginIndexOutputTarget(useCase.repositoryRoot, request.OutputPath, index)
	if err != nil {
		logPluginIndexRefusal(request.Mode, "target validation")
		return pluginIndexCommandResult{}, err
	}
	result.Target = target
	log.PluginV(log.Exec, "Preparing atomic Plugin Index write")
	if err := useCase.persister.Persist(target.AbsolutePath, output); err != nil {
		logPluginIndexRefusal(request.Mode, "atomic write")
		return pluginIndexCommandResult{}, err
	}
	log.PluginV(log.Exec, "Plugin Index write completed")
	log.PluginV(log.Preflight, "Confirming Plugin Index persistence result")
	log.PluginV(log.Exec, "Plugin Index persistence completed")
	return result, nil
}

func logPluginIndexStart(mode pluginIndexCommandMode) {
	switch mode {
	case pluginIndexCheckMode:
		log.PluginV(log.Config, "Resolving Plugin Index source")
		log.PluginV(log.Config, "Deriving Plugin Index")
	case pluginIndexPersistMode:
		log.PluginV(log.Config, "Resolving Plugin Index source")
		log.PluginV(log.Config, "Constructing Plugin Index")
	}
}

func logPluginIndexValidation(mode pluginIndexCommandMode) {
	if mode == pluginIndexRenderMode {
		return
	}
	log.PluginV(log.Preflight, "Validating Plugin Index schema")
	log.PluginV(log.Preflight, "Validating Plugin Index repositories")
	log.PluginV(log.Preflight, "Validating Plugin Index plugins")
}

func logPluginIndexRefusal(mode pluginIndexCommandMode, phase string) {
	switch mode {
	case pluginIndexCheckMode:
		log.PluginV(log.Exec, "Plugin Index validation refused during %s", phase)
	case pluginIndexPersistMode:
		log.PluginV(log.Exec, "Plugin Index persistence refused during %s", phase)
	}
}
