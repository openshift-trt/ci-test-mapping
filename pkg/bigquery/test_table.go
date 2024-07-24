package bigquery

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"

	v1 "github.com/openshift-eng/ci-test-mapping/pkg/api/types/v1"
)

type TestTableManager struct {
	ctx        context.Context
	junitTable string
	client     *Client
	config     *v1.Config
	dataset    string
}

func NewTestTableManager(ctx context.Context, client *Client, config *v1.Config, junitTable string) *TestTableManager {
	return &TestTableManager{
		ctx:        ctx,
		config:     config,
		junitTable: junitTable,
		client:     client,
	}
}

func (tm *TestTableManager) ListTests() ([]v1.TestInfo, error) {
	now := time.Now()
	log.Infof("fetching unique test/suite names from bigquery")

	q := tm.client.bigquery.Query(tm.buildSQLQuery())
	it, err := q.Read(tm.ctx)
	if err != nil {
		return nil, err
	}

	var results []v1.TestInfo
	for {
		var testInfo v1.TestInfo
		err := it.Next(&testInfo)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, testInfo)
	}
	log.WithFields(log.Fields{
		"count": len(results),
	}).Infof("fetched unique test/suite names from bigquery in %v", time.Since(now))

	return results, nil
}

func (tm *TestTableManager) buildSQLQuery() string {
	var suitesFilter, jobsFilter, excludeSuitesFilter, excludeTestsFilter, excludeJobsFilter string

	table := tm.client.bigquery.Dataset(tm.dataset).Table(tm.junitTable)

	if len(tm.config.IncludeSuites) > 0 {
		suitesFilter = fmt.Sprintf("testsuite IN ('%s')", strings.Join(tm.config.IncludeSuites, "','"))
	} else {
		suitesFilter = "1=1" // no filtering by suites
	}

	if len(tm.config.IncludeJobs) > 0 {
		jobsFilter = fmt.Sprintf("AND (%s)", strings.Join(func(jobs []string) []string {
			var filters []string
			for _, job := range jobs {
				filters = append(filters, fmt.Sprintf("prowjob_name LIKE '%s'", job))
			}
			return filters
		}(tm.config.IncludeJobs), " OR "))
	}

	if len(tm.config.ExcludeSuites) > 0 {
		excludeSuitesFilter = fmt.Sprintf("AND testsuite NOT IN ('%s')", strings.Join(tm.config.ExcludeSuites, "','"))
	}

	if len(tm.config.ExcludeTests) > 0 {
		excludeTestsFilter = fmt.Sprintf("AND test_name NOT LIKE '%s'", strings.Join(tm.config.ExcludeTests, "' AND test_name NOT LIKE '"))
	}

	if len(tm.config.ExcludeJobs) > 0 {
		excludeJobsFilter = fmt.Sprintf("AND prowjob_name NOT LIKE '%s'", strings.Join(tm.config.ExcludeJobs, "' AND prowjob_name NOT LIKE '"))
	}

	sql := fmt.Sprintf(`
		SELECT DISTINCT
		    test_name as name,
		    testsuite as suite
		FROM
			%s.%s.%s
		WHERE
		    %s
		%s
		%s
		%s
		%s
		AND
		    modified_time <= CURRENT_DATETIME()
		ORDER BY name, testsuite DESC`,
		tm.client.projectName, tm.client.datasetName, table.TableID, suitesFilter, jobsFilter, excludeSuitesFilter, excludeTestsFilter, excludeJobsFilter)

	log.Debugf("query is %s", sql)
	return sql
}
