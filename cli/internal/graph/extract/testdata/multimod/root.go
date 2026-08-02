package root

import "other.example.com/service"

// Boot starts the application.
func Boot() string { return service.Name() }
