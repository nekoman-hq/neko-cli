package pluginindex

import "context"

type pluginIndexCommandResult struct {
	Repository string
	OutputPath string
	RawOutput  string
	Mode       pluginIndexCommandMode
	Plugins    int
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
	index, err := useCase.query.Query(ctx, GenerateOptions{Root: useCase.repositoryRoot, Repository: request.Repository})
	if err != nil {
		return pluginIndexCommandResult{}, err
	}
	result := pluginIndexCommandResult{
		Repository: index.Repository,
		OutputPath: request.OutputPath,
		Mode:       request.Mode,
		Plugins:    len(index.Plugins),
	}
	if request.Mode == pluginIndexCheckMode {
		return result, nil
	}

	output, err := useCase.builder.Build(index, WriteOptions{Pretty: request.Pretty})
	if err != nil {
		return pluginIndexCommandResult{}, err
	}
	if request.Mode == pluginIndexRenderMode {
		result.RawOutput = string(output)
		return result, nil
	}
	if err := useCase.persister.Persist(request.OutputPath, output); err != nil {
		return pluginIndexCommandResult{}, err
	}
	return result, nil
}
