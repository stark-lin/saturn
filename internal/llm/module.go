// This file initializes the LLM module dependencies.
package llm

type Module struct {
	Handler *Handler
	Service *Service
}

func NewModule(deps ServiceDependencies) Module {
	service := NewService(deps)
	return Module{
		Handler: NewHandler(service),
		Service: service,
	}
}
