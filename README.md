# SES Email Templates Manager

This project provides a simple application written in Go that manages email templates on AWS Simple Email Service (SES). The application watches local template files and a configuration file for changes, and updates the corresponding templates on AWS SES.

## Features

- Watches local email template files for changes and automatically updates the corresponding templates on AWS SES.
- If a template does not exist on AWS SES, the application creates it.
- Also watches the configuration file for changes and updates the watched template files accordingly.
- Only updates templates on AWS SES if the template's subject, text, or HTML part has changed.

## Requirements

- AWS account with access to SES.
- Appropriate AWS credentials.
- Go 1.19.
- Dependencies: [AWS SDK for Go](https://github.com/aws/aws-sdk-go), [Watcher](https://github.com/radovskyb/watcher).
- Sentry for error logging (should be simple to disable).

## Usage

1. Clone this repository:
```
git clone https://github.com/Mihonarium/SES_Template_Manager.git
```
2. Install the dependencies:
```
go get ./...
```
3. Set up your configuration file (by default `/home/ubuntu/ses_config.json`) with your AWS credentials, the AWS region, and the list of templates to watch. See the provided `ses_config.example.json` for an example.
4. Run the application:
``` 
go run main.go
```
or build it and run the executable:
```
go build -o ses_emails main.go
./ses_emails
```

The application will now start watching the template files specified in your configuration file, and will update the corresponding templates on AWS SES whenever a file changes.

You can also specify the path to the configuration file as a command line argument:
```
Usage of ./ses_emails:
  -config string
        Full path to the config file (default "/home/ubuntu/ses_config.json")
```

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## Acknowledgements

More than half of the code and this README was written by GPT-4.

## License

[MIT](LICENSE)
