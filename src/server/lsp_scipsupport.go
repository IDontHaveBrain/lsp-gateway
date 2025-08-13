package server

import "lsp-gateway/src/server/scip"

func (m *LSPManager) getScipStorage() scip.SCIPDocumentStorage {
    if m == nil || m.scipCache == nil {
        return nil
    }
    return m.scipCache.GetSCIPStorage()
}

