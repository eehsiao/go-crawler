// Author :		Eric<eehsiao@gmail.com>
package jobctrl

import (
	"sync"
)

type JobCtrl struct {
	jobLock sync.RWMutex
	jobCnt  int
	maxJobs int
}

func NewJobCtrl(maxJobs int) *JobCtrl {
	return &JobCtrl{
		jobCnt:  0,
		maxJobs: maxJobs,
	}
}

func (j *JobCtrl) IncJob() bool {
	j.jobLock.Lock()
	defer j.jobLock.Unlock()

	if j.jobCnt >= j.maxJobs {
		return false
	}

	j.jobCnt++

	return true
}

func (j *JobCtrl) DecJob() bool {
	j.jobLock.Lock()
	defer j.jobLock.Unlock()

	j.jobCnt--

	return true
}

func (j *JobCtrl) GetJobCount() int {
	j.jobLock.RLock()
	defer j.jobLock.RUnlock()

	return j.jobCnt
}
