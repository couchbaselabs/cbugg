basePath = '../';

files = [
  JASMINE,
  JASMINE_ADAPTER,
  'static/lib/angular.min.js',
  'static/lib/angular-*.js',
  'static/js/**/*.js',
  'test/unit/**/*.js'
];

singleRun = true;

browsers = ['Firefox'];

reporters = ['dots'];