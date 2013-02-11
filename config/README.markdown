# Configuring Elasticsearch Index for cbugg

1.  Create the index using the supplied index configuration JSON file:

    curl -XPUT http://<elasticsearch hostname>:9200/cbugg -d @cbugg-index.json

 # Testing cbugg JavaScript

 1.  Install Testacular [http://vojtajina.github.com/testacular/](http://vojtajina.github.com/testacular/)

     npm install -g testacular
 2.  Run Testacular with the provided config file

     testacular start config/testacular.conf.js