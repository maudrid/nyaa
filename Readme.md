# Nyaa
My first Go application.
Reads several filter configurations from a config file.
Make HTTP requests to the base url defined in the config, adding a filter for each request.
Concatenates all the responses into a single RSS feed

## Usage
Run the application with a suitable config file
Access the `/links` path from an HTTP client.
Chances to the filters and base URL settings are applied on save.
No need to restart the app.

A Dockerfile is included for running this as a docker service.
Mount the config file to `/conf.yml` or specify your own `CMD`
