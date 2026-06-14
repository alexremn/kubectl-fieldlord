// Package ownership decodes Kubernetes managedFields into a per-field ownership model.
package ownership

// Operation is the managedFields operation: Apply or Update.
type Operation string

const (
	OperationApply  Operation = "Apply"
	OperationUpdate Operation = "Update"
)

// Owner uniquely identifies one managedFields entry. A single manager name may
// appear as several Owners disambiguated by Operation, APIVersion, and Subresource.
type Owner struct {
	Manager     string    `json:"manager"`
	Operation   Operation `json:"operation"`
	APIVersion  string    `json:"apiVersion"`
	Subresource string    `json:"subresource,omitempty"`
	Time        string    `json:"time,omitempty"` // RFC3339, second precision, UTC; may be empty
}

// OwnedPath is one field path and the owners that claim it.
type OwnedPath struct {
	Path       string  `json:"path"`
	Atomic     bool    `json:"atomic"`
	MultiOwner bool    `json:"multiOwner"`
	Owners     []Owner `json:"owners"`
}

// Model is the ownership view of a single object.
type Model struct {
	Paths    []OwnedPath `json:"-"`
	Warnings []string    `json:"-"`
}
