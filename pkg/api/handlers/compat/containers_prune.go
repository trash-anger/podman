package compat

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	lpfilters "github.com/containers/libpod/libpod/filters"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	var (
		delContainers []string
		space         int64
		filterFuncs   []libpod.ContainerFilter
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Filters map[string][]string `schema:"filters"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	for k, v := range query.Filters {
		for _, val := range v {
			generatedFunc, err := lpfilters.GenerateContainerFilterFuncs(k, val, runtime)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}
	prunedContainers, pruneErrors, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Libpod response differs
	if utils.IsLibpodRequest(r) {
		report := &entities.ContainerPruneReport{
			Err: pruneErrors,
			ID:  prunedContainers,
		}
		utils.WriteResponse(w, http.StatusOK, report)
		return
	}
	for ctrID, size := range prunedContainers {
		if pruneErrors[ctrID] == nil {
			space += size
			delContainers = append(delContainers, ctrID)
		}
	}
	report := types.ContainersPruneReport{
		ContainersDeleted: delContainers,
		SpaceReclaimed:    uint64(space),
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
