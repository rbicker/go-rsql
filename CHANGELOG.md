# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2021-08-01
### Changed
* the equal query operator for mongodb is formatted as { "key": value } instead of { "key": { "$eq": value } } to provide more flexibility 

## [0.3.0] - 2021-02-17
### Added
* added process options to define to allow or disallow specific keys.
* added key transformers.

## [0.2.0] - 2020-05-29
### Changed
* mongodb related logical and query operators are now provided as "formatter-functions".
This makes the parser much more flexible, however the `Mongo()`-function needs to be passed to `NewParser()` from now on. 

## [0.1.0] - 2020-05-28
### Changed
* custom operators can now define MongoFormatter as a function instead of a MongoOperator string to increase the flexibility
### Fixed
* allow nested parentheses, so it is possible to define filters like: userIds=in=(ObjectId("xxx"),ObjectId("yyy"))

## [0.0.2] - 2020-05-23
### Added
* added the ability to handle empty rsql strings

## [0.0.1] - 2020-05-16
### Added
* parser to parse rsql string to mongodb json filter string 
* functionality to add custom operators