package publishing

// Test hooks (compiled only into this package's tests).

func SetAfterLock(s *Service, fn func(jobID string))          { s.hooks.afterLock = fn }
func SetBeforeCommit(s *Service, fn func(jobID string) error) { s.hooks.beforeCommit = fn }
