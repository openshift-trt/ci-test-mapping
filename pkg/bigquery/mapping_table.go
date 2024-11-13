package bigquery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	v1 "github.com/openshift-eng/ci-test-mapping/pkg/api/types/v1"
)

type MappingTableManager struct {
	ctx              context.Context
	mappingTableName string
	client           *Client
	schema           bigquery.Schema
}

type TestMappingTableManager struct {
	MappingTableManager
}

func NewTestMappingTableManager(ctx context.Context, client *Client, mappingTable string, schema bigquery.Schema) *TestMappingTableManager {
	return &TestMappingTableManager{
		MappingTableManager{
			ctx:              ctx,
			mappingTableName: mappingTable,
			client:           client,
			schema:           schema,
		},
	}
}

type VariantMappingTableManager struct {
	MappingTableManager
}

func NewVariantMappingTableManager(ctx context.Context, client *Client, mappingTable string, schema bigquery.Schema) *VariantMappingTableManager {
	return &VariantMappingTableManager{
		MappingTableManager{
			ctx:              ctx,
			mappingTableName: mappingTable,
			client:           client,
			schema:           schema,
		},
	}
}

func (m *MappingTableManager) Migrate() error {
	dataset := m.client.bigquery.Dataset(m.client.datasetName)
	table := dataset.Table(m.mappingTableName)

	md, err := table.Metadata(m.ctx)
	// Create table if it doesn't exist
	if gbErr, ok := err.(*googleapi.Error); err != nil && ok && gbErr.Code == 404 {
		log.Infof("table doesn't existing, creating table %q", m.mappingTableName)
		if err := table.Create(m.ctx, &bigquery.TableMetadata{
			Schema: v1.TestMappingTableSchema,
		}); err != nil {
			return err
		}
		log.Infof("table created %q", m.mappingTableName)
	} else if err != nil {
		return err
	} else {
		if !schemasEqual(md.Schema, m.schema) {
			if _, err := table.Update(m.ctx, bigquery.TableMetadataToUpdate{Schema: v1.TestMappingTableSchema}, md.ETag); err != nil {
				log.WithError(err).Errorf("failed to update table schema for %q", m.mappingTableName)
				return err
			}
			log.Infof("table schema updated %q", m.mappingTableName)
		} else {
			log.Infof("table schema is up-to-date %q", m.mappingTableName)
		}
	}

	return nil
}

func (m *MappingTableManager) PruneMappings() error {
	now := time.Now()
	log.Infof("pruning mappings from bigquery")
	table := m.client.bigquery.Dataset(m.client.datasetName).Table(m.mappingTableName)

	tableLocator := fmt.Sprintf("%s.%s.%s", table.ProjectID, m.client.datasetName, table.TableID)

	sql := fmt.Sprintf(`DELETE FROM %s WHERE created_at < (SELECT MAX(created_at) FROM %s)`, tableLocator, tableLocator)
	log.Infof("query is %q", sql)

	q := m.client.bigquery.Query(sql)
	_, err := q.Read(m.ctx)
	log.Infof("pruned mapping table in %+v", time.Since(now))
	if err != nil && strings.Contains(err.Error(), "streaming") {
		log.Warningf("got error while trying to prune the table; please wait 90 minutes and try again. You cannot prune after modifying the table.")
	}
	return err
}

func (m *MappingTableManager) Table() *bigquery.Table {
	dataset := m.client.bigquery.Dataset(m.client.datasetName)
	return dataset.Table(m.mappingTableName)
}

func schemasEqual(a, b bigquery.Schema) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name ||
			a[i].Type != b[i].Type ||
			a[i].Repeated != b[i].Repeated ||
			a[i].Required != b[i].Required {
			return false
		}
	}

	return true
}

func (tm *TestMappingTableManager) ListTestMappings() ([]v1.TestOwnership, error) {
	now := time.Now()
	log.Infof("fetching mappings from bigquery")
	table := tm.client.bigquery.Dataset(tm.client.datasetName).Table(tm.mappingTableName + "_latest") // use the view

	sql := fmt.Sprintf(`
		SELECT 
		    *
		FROM
			%s.%s.%s`,
		table.ProjectID, tm.client.datasetName, table.TableID)
	log.Debugf("query is %q", sql)

	q := tm.client.bigquery.Query(sql)
	it, err := q.Read(tm.ctx)
	if err != nil {
		return nil, err
	}

	var results []v1.TestOwnership
	for {
		var testOwnership v1.TestOwnership
		err := it.Next(&testOwnership)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, testOwnership)
	}
	log.Infof("fetched %d mapping from bigquery in %v", len(results), time.Since(now))

	return results, nil
}

func (tm *TestMappingTableManager) PushTestMappings(mappings []v1.TestOwnership) error {
	var batchSize = 500

	table := tm.client.bigquery.Dataset(tm.client.datasetName).Table(tm.mappingTableName)
	inserter := table.Inserter()
	for i := 0; i < len(mappings); i += batchSize {
		end := i + batchSize
		if end > len(mappings) {
			end = len(mappings)
		}

		if err := inserter.Put(tm.ctx, mappings[i:end]); err != nil {
			return err
		}
		log.Infof("added %d rows to mapping bigquery table", end-i)
	}

	return nil
}

func (vm *VariantMappingTableManager) ListVariantMappings() ([]v1.VariantMapping, error) {
	now := time.Now()
	log.Infof("fetching variant mappings from bigquery")
	table := vm.client.bigquery.Dataset(vm.client.datasetName).Table(vm.mappingTableName + "_latest") // use the view

	sql := fmt.Sprintf(`
		SELECT
		    *
		FROM
			%s.%s.%s`,
		table.ProjectID, vm.client.datasetName, table.TableID)
	log.Debugf("query is %q", sql)

	q := vm.client.bigquery.Query(sql)
	it, err := q.Read(vm.ctx)
	if err != nil {
		return nil, err
	}

	var results []v1.VariantMapping
	for {
		var mapping v1.VariantMapping
		err := it.Next(&mapping)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, mapping)
	}
	log.Infof("fetched %d variant mapping from bigquery in %v", len(results), time.Since(now))

	return results, nil
}

func (vm *VariantMappingTableManager) PushVariantMappings(mappings []v1.VariantMapping) error {
	var batchSize = 500

	table := vm.client.bigquery.Dataset(vm.client.datasetName).Table(vm.mappingTableName)
	inserter := table.Inserter()
	for i := 0; i < len(mappings); i += batchSize {
		end := i + batchSize
		if end > len(mappings) {
			end = len(mappings)
		}

		if err := inserter.Put(vm.ctx, mappings[i:end]); err != nil {
			return err
		}
		log.Infof("added %d rows to mapping bigquery table", end-i)
	}

	return nil
}
