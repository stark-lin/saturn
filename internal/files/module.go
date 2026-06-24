// This file initializes the Files module dependencies.
package files

import platformdb "github.com/stark-lin/saturn/internal/platform/db"

type Module struct {
	Handler *Handler
	Service *Service
}

func NewModule(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
	storageService StorageService,
) Module {
	service := NewService(repo, transactions, references, auditService, storageService)
	return Module{
		Handler: NewHandler(service),
		Service: service,
	}
}
