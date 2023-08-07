# Exporter Password Change

## 00--create-cluster:
The TestStep will:

1) Apply the `files/inital-postgrescluster.yaml` file to create a cluster with monitoring enabled
2) Assert that conditions outlined in `files/initial-postgrescluster-checks.yaml` are met
    - PostgresCluster exists with a single ready replica
    - A pod with `cluster` and `crunchy-postgres-exporter` labels has the status `{phase: Running}`
    - A `<cluster>-monitoring` secret exists with correct labels and ownerReferences

## 00-assert:

This TestAssert will loop through a script until:
1) the instance pod has the `ContainersReady` condition with status `true`
2) the asserts from `00--create-cluster` are met.

If this step fails, we attempt to collect the describe output for the instance pod.

## 01-assert:

This TestAssert will loop through a script until:
1) The metrics endpoint returns `pg_exporter_last_scrape_error 0` meaning the exporter was able to access postgres metrics

If this step fails logs are collected from the exporter container.

## 02-change-password:

This TestStep will:
1) Apply the `files/update-monitoring-password.yaml` file to set the monitoring password to `password`
2) Assert that conditions outlined in `files/update-monitoring-password-checks.yaml` are met
    - A `<cluster>-monitoring` secret exists with `data.password` set to the encoded value for `password`

## 03-restart-exporter:

This TestStep will:
1) Run a command to delete the instance pod - this will cause the exporter container to restart with the updated password
2) Assert that the instance pod exists with the status `{phase: Running}`

## 04-assert:

This TestAssert will loop through a script until:
1) An exec command can confirm that the `DATA_SOURCE_PASS` environment variable contains the updated password
2) The instance pod has the `ContainersReady` condition with status `true`
3) The metrics endpoint returns `pg_exporter_last_scrape_error 0` meaning the exporter was able to access postgres metrics using the updated password

If this step fails logs are collected from the exporter container.