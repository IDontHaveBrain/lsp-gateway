package scip

import (
	"context"
	"fmt"

	"lsp-gateway/src/internal/types"
)

// GetDocumentURIFromOccurrence extracts the document URI for a given SCIP occurrence.
// It uses an efficient lookup for SimpleSCIPStorage, otherwise falls back to searching all documents.
func GetDocumentURIFromOccurrence(storage SCIPDocumentStorage, occ *SCIPOccurrence) (string, error) {
	if occ == nil {
		return "", fmt.Errorf("occurrence cannot be nil")
	}

	// If storage is SimpleSCIPStorage, use its efficient method
	if simpleStorage, ok := storage.(*SimpleSCIPStorage); ok {
		uri := simpleStorage.extractDocumentURIFromOccurrence(*occ)
		if uri == "" {
			return "", fmt.Errorf("document URI not found for occurrence at %d:%d",
				occ.Range.Start.Line, occ.Range.Start.Character)
		}
		return uri, nil
	}

	// Fallback: search through all documents to find the occurrence
	ctx := context.Background()
	documents, err := storage.ListDocuments(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list documents: %w", err)
	}

	for _, uri := range documents {
		doc, err := storage.GetDocument(ctx, uri)
		if err != nil {
			continue // Skip documents that can't be retrieved
		}

		// Check if this occurrence exists in this document
		for _, docOcc := range doc.Occurrences {
			if occurrencesEqual(*occ, docOcc) {
				return uri, nil
			}
		}
	}

	return "", fmt.Errorf("document URI not found for occurrence at %d:%d",
		occ.Range.Start.Line, occ.Range.Start.Character)
}

// CreateLocationFromOccurrence creates a types.Location from a SCIP occurrence and its storage.
// It resolves the document URI and combines it with the occurrence's range.
func CreateLocationFromOccurrence(storage SCIPDocumentStorage, occ *SCIPOccurrence) (*types.Location, error) {
	if occ == nil {
		return nil, fmt.Errorf("occurrence cannot be nil")
	}

	uri, err := GetDocumentURIFromOccurrence(storage, occ)
	if err != nil {
		return nil, fmt.Errorf("failed to get document URI: %w", err)
	}

	return &types.Location{
		URI:   uri,
		Range: occ.Range,
	}, nil
}

// occurrencesEqual compares two SCIP occurrences for equality based on symbol and position
func occurrencesEqual(occ1, occ2 SCIPOccurrence) bool {
	return occ1.Symbol == occ2.Symbol &&
		occ1.Range.Start.Line == occ2.Range.Start.Line &&
		occ1.Range.Start.Character == occ2.Range.Start.Character &&
		occ1.Range.End.Line == occ2.Range.End.Line &&
		occ1.Range.End.Character == occ2.Range.End.Character
}
