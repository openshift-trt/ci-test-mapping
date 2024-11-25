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
)

type MappingTableManager[T interface{}] struct {
	ctx              context.Context
	mappingTableName string
	client           *Client
	schema           bigquery.Schema
}

func NewMappingTableManager[T interface{}](ctx context.Context, client *Client, mappingTable string, schema bigquery.Schema) *MappingTableManager[T] {
	return &MappingTableManager[T]{
		ctx:              ctx,
		mappingTableName: mappingTable,
		client:           client,
		schema:           schema,
	}
}

func (m *MappingTableManager[T]) Migrate() error {
	dataset := m.client.bigquery.Dataset(m.client.datasetName)
	table := dataset.Table(m.mappingTableName)

	md, err := table.Metadata(m.ctx)
	// Create table if it doesn't exist
	if gbErr, ok := err.(*googleapi.Error); err != nil && ok && gbErr.Code == 404 {
		log.Infof("table doesn't exist, creating table %q", m.mappingTableName)
		if err := table.Create(m.ctx, &bigquery.TableMetadata{
			Schema: m.schema,
		}); err != nil {
			return err
		}
		log.Infof("table created %q", m.mappingTableName)
	} else if err != nil {
		return err
	} else {
		if !schemasEqual(md.Schema, m.schema) {
			if _, err := table.Update(m.ctx, bigquery.TableMetadataToUpdate{Schema: m.schema}, md.ETag); err != nil {
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

func (m *MappingTableManager[T]) ListMappings() ([]T, error) {
	now := time.Now()
	log.Infof("fetching mappings from bigquery")
	table := m.client.bigquery.Dataset(m.client.datasetName).Table(m.mappingTableName + "_latest") // use the view

	sql := fmt.Sprintf(`
		SELECT
		    *
		FROM
			%s.%s.%s`,
		table.ProjectID, m.client.datasetName, table.TableID)
	log.Debugf("query is %q", sql)

	q := m.client.bigquery.Query(sql)
	it, err := q.Read(m.ctx)
	if err != nil {
		return nil, err
	}

	var results []T
	for {
		var testOwnership T
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

func (m *MappingTableManager[T]) PushMappings(mappings []T) error {
	var batchSize = 500

	table := m.client.bigquery.Dataset(m.client.datasetName).Table(m.mappingTableName)
	inserter := table.Inserter()
	for i := 0; i < len(mappings); i += batchSize {
		end := i + batchSize
		if end > len(mappings) {
			end = len(mappings)
		}

		if err := inserter.Put(m.ctx, mappings[i:end]); err != nil {
			return err
		}
		log.Infof("added %d rows to mapping bigquery table", end-i)
	}

	return nil
}

func (m *MappingTableManager[T]) PruneMappings() error {
	now := time.Now()
	log.Infof("pruning mappings from bigquery table %s", m.mappingTableName)
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

func (m *MappingTableManager[T]) Table() *bigquery.Table {
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
