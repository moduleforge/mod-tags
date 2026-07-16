package httpapi

import (
	"log/slog"

	coredb "github.com/moduleforge/core-model/db"
	"github.com/moduleforge/tags-api/service"
)

// NewDeps constructs a Deps value from its components. mfgen's codegen
// template always emits constructor references as a function call
// (`Constructor(args...)`) — never a composite literal — so this must be a
// real function, not a bare struct type, for moduleforge.module.yaml's
// tagsDeps entry (`constructor: tagshttpapi.NewDeps`) to generate valid Go.
func NewDeps(coreQuerier coredb.Querier, services *service.Services, logger *slog.Logger) Deps {
	return Deps{CoreQuerier: coreQuerier, Services: services, Logger: logger}
}
