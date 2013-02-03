# Configuring Elasticsearch

1.  Create the index using the supplied index configuration JSON file:

    curl -XPUT http://<elasticsearch hostname>:9200/cbugg -d @cbugg-index.json
