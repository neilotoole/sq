package sqlserver

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// getDBProperties returns the properties for db. These are a union
// of the SERVERPROPERTY values with those from sys.configurations.
func getDBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	m1, err := getSysConfigurations(ctx, db)
	if err != nil {
		return nil, err
	}

	m2, err := getServerProperties(ctx, db)
	if err != nil {
		return nil, err
	}

	for k, v := range m2 {
		m1[k] = v
	}

	return m1, nil
}

// getSysConfigurations reads values from sys.configurations.
func getSysConfigurations(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	const configQuery = `SELECT name, value FROM sys.configurations ORDER BY name`

	m := map[string]any{}
	rows, err := db.QueryContext(ctx, configQuery)
	if err != nil {
		return nil, errw(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDBRows, rows)

	for rows.Next() {
		var name string
		var val int

		if err = rows.Scan(&name, &val); err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		progress.DebugSleep(ctx)

		m[name] = val
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return m, nil
}

// getServerProperties returns the known SERVERPROPERTY values.
// See:
// - https://gist.github.com/RealWorldDevelopers/651273796d81f3521e9016533876ba66
// - https://database.guide/quick-script-that-returns-all-properties-from-serverproperty-in-sql-server-2017-2019/
// - https://learn.microsoft.com/en-us/sql/t-sql/functions/serverproperty-transact-sql?view=sql-server-ver16
func getServerProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	m := map[string]any{}
	rows, err := db.QueryContext(ctx, serverPropertiesQuery)
	if err != nil {
		return nil, errw(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDBRows, rows)

	for rows.Next() {
		var name string
		var val any

		if err = rows.Scan(&name, &val); err != nil {
			return nil, errw(err)
		}

		if val == nil {
			continue
		}
		progress.Incr(ctx, 1)
		progress.DebugSleep(ctx)

		m[name] = val
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return m, nil
}

// https://learn.microsoft.com/en-us/sql/t-sql/functions/serverproperty-transact-sql
const serverPropertiesQuery = `SELECT 'BuildClrVersion' AS name, SERVERPROPERTY('BuildClrVersion') AS value
UNION ALL
SELECT 'Collation', SERVERPROPERTY('Collation')
UNION ALL
SELECT 'CollationID', SERVERPROPERTY('CollationID')
UNION ALL
SELECT 'ComparisonStyle', SERVERPROPERTY('ComparisonStyle')
UNION ALL
SELECT 'ComputerNamePhysicalNetBIOS', SERVERPROPERTY('ComputerNamePhysicalNetBIOS')
UNION ALL
SELECT 'Edition', SERVERPROPERTY('Edition')
UNION ALL
SELECT 'EditionID', SERVERPROPERTY('EditionID')
UNION ALL
SELECT 'EngineEdition', SERVERPROPERTY('EngineEdition')
UNION ALL
SELECT 'FilestreamConfiguredLevel', SERVERPROPERTY('FilestreamConfiguredLevel')
UNION ALL
SELECT 'FilestreamEffectiveLevel', SERVERPROPERTY('FilestreamEffectiveLevel')
UNION ALL
SELECT 'FilestreamShareName', SERVERPROPERTY('FilestreamShareName')
UNION ALL
SELECT 'HadrManagerStatus', SERVERPROPERTY('HadrManagerStatus')
UNION ALL
SELECT 'InstanceDefaultBackupPath', SERVERPROPERTY('InstanceDefaultBackupPath')
UNION ALL
SELECT 'InstanceDefaultDataPath', SERVERPROPERTY('InstanceDefaultDataPath')
UNION ALL
SELECT 'InstanceDefaultLogPath', SERVERPROPERTY('InstanceDefaultLogPath')
UNION ALL
SELECT 'InstanceName', SERVERPROPERTY('InstanceName')
UNION ALL
SELECT 'IsAdvancedAnalyticsInstalled', SERVERPROPERTY('IsAdvancedAnalyticsInstalled')
UNION ALL
SELECT 'IsBigDataCluster', SERVERPROPERTY('IsBigDataCluster')
UNION ALL
SELECT 'IsClustered', SERVERPROPERTY('IsClustered')
UNION ALL
SELECT 'IsExternalAuthenticationOnly', SERVERPROPERTY('IsExternalAuthenticationOnly')
UNION ALL
SELECT 'IsExternalGovernanceEnabled', SERVERPROPERTY('IsFullTextIsExternalGovernanceEnabledInstalled')
UNION ALL
SELECT 'IsFullTextInstalled', SERVERPROPERTY('IsFullTextInstalled')
UNION ALL
SELECT 'IsHadrEnabled', SERVERPROPERTY('IsHadrEnabled')
UNION ALL
SELECT 'IsIntegratedSecurityOnly', SERVERPROPERTY('IsIntegratedSecurityOnly')
UNION ALL
SELECT 'IsLocalDB', SERVERPROPERTY('IsLocalDB')
UNION ALL
SELECT 'IsPolyBaseInstalled', SERVERPROPERTY('IsPolyBaseInstalled')
UNION ALL
SELECT 'IsServerSuspendedForSnapshotBackup', SERVERPROPERTY('IsServerSuspendedForSnapshotBackup')
UNION ALL
SELECT 'IsSingleUser', SERVERPROPERTY('IsSingleUser')
UNION ALL
SELECT 'IsTempDbMetadataMemoryOptimized', SERVERPROPERTY('IsTempDbMetadataMemoryOptimized')
UNION ALL
SELECT 'IsXTPSupported', SERVERPROPERTY('IsXTPSupported')
UNION ALL
SELECT 'LCID', SERVERPROPERTY('LCID')
UNION ALL
SELECT 'LicenseType', SERVERPROPERTY('LicenseType')
UNION ALL
SELECT 'MachineName', SERVERPROPERTY('MachineName')
UNION ALL
SELECT 'NumLicenses', SERVERPROPERTY('NumLicenses')
UNION ALL
SELECT 'PathSeparator', SERVERPROPERTY('PathSeparator')
UNION ALL
SELECT 'ProcessID', SERVERPROPERTY('ProcessID')
UNION ALL
SELECT 'ProductBuild', SERVERPROPERTY('ProductBuild')
UNION ALL
SELECT 'ProductBuildType', SERVERPROPERTY('ProductBuildType')
UNION ALL
SELECT 'ProductLevel', SERVERPROPERTY('ProductLevel')
UNION ALL
SELECT 'ProductMajorVersion', SERVERPROPERTY('ProductMajorVersion')
UNION ALL
SELECT 'ProductMinorVersion', SERVERPROPERTY('ProductMinorVersion')
UNION ALL
SELECT 'ProductUpdateLevel', SERVERPROPERTY('ProductUpdateLevel')
UNION ALL
SELECT 'ProductUpdateReference', SERVERPROPERTY('ProductUpdateReference')
UNION ALL
SELECT 'ProductVersion', SERVERPROPERTY('ProductVersion')
UNION ALL
SELECT 'ResourceLastUpdateDateTime', SERVERPROPERTY('ResourceLastUpdateDateTime')
UNION ALL
SELECT 'ResourceVersion', SERVERPROPERTY('ResourceVersion')
UNION ALL
SELECT 'ServerName', SERVERPROPERTY('ServerName')
UNION ALL
SELECT 'SqlCharSet', SERVERPROPERTY('SqlCharSet')
UNION ALL
SELECT 'SqlCharSetName', SERVERPROPERTY('SqlCharSetName')
UNION ALL
SELECT 'SqlSortOrder', SERVERPROPERTY('SqlSortOrder')
UNION ALL
SELECT 'SqlSortOrderName', SERVERPROPERTY('SqlSortOrderName')
UNION ALL
SELECT 'SuspendedDatabaseCount', SERVERPROPERTY('SuspendedDatabaseCount')
;`
