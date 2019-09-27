// Author :		Eric<eehsiao@gmail.com>
package jobctrl

import (
	"sync"
)

type jobCtrl struct {
	jobLock sync.RWMutex
	jobCnt  int
	maxJobs int
}

func NewJobCtrl(maxJobs int) *jobCtrl {
	return &jobCtrl{
		jobCnt:  0,
		maxJobs: maxJobs,
	}
}

func (j *jobCtrl) IncJob() bool {
	j.jobLock.Lock()
	defer j.jobLock.Unlock()

	if j.jobCnt >= j.maxJobs {
		return false
	}

	j.jobCnt++

	return true
}

func (j *jobCtrl) DecJob() bool {
	j.jobLock.Lock()
	defer j.jobLock.Unlock()

	j.jobCnt--

	return true
}

func (j *jobCtrl) GetJobCount() int {
	j.jobLock.RLock()
	defer j.jobLock.RUnlock()

	return j.jobCnt
}
