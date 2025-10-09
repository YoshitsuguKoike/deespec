package run

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
)

// SaveStateAndJournalTX saves state.json and appends journal entry atomically using transaction
func SaveStateAndJournalTX(
	state *common.State,
	journalRec map[string]interface{},
	paths app.Paths,
	prevVersion int,
) error {
	// Create service
	txService := service.NewExecutionTransactionService(common.Warn, common.GetGlobalConfig())

	// Convert CLI State to Service State
	svcState := &service.State{
		Version:        state.Version,
		Current:        state.Current,
		Status:         state.Status,
		Turn:           state.Turn,
		WIP:            state.WIP,
		LeaseExpiresAt: state.LeaseExpiresAt,
		Inputs:         state.Inputs,
		LastArtifacts:  state.LastArtifacts,
		Decision:       state.Decision,
		Attempt:        state.Attempt,
		Meta: service.StateMeta{
			UpdatedAt: state.Meta.UpdatedAt,
		},
	}

	// Execute service
	err := txService.SaveStateAndJournalTX(svcState, journalRec, paths, prevVersion)

	// Sync back the state changes (version increment)
	state.Version = svcState.Version
	state.Meta.UpdatedAt = svcState.Meta.UpdatedAt

	return err
}
