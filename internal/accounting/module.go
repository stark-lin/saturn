// This file initializes the Accounting module dependencies.
package accounting

import platformdb "github.com/stark-lin/go-proj/internal/platform/db"

type Module struct {
	Handler *Handler
	Service *Service
}

func NewModule(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
) Module {
	service := NewService(repo, transactions, references, auditService)
	return Module{
		Handler: NewHandler(service),
		Service: service,
	}
}
