# WOK

This is application for personal use (1 person - me).

This is a project when looking for a new job. Simple application with list of applications.
This application is build for keeping track of CV sent (different CV with different information) - don't want to overlap with two different CV sent.

Applications contains:
- name of company
- aliases for company
- date of CV sent
- CV version (versioning by date)
- screenshots of job description
- link to job description

Applications list can be searched by company name or alias.
Some companies have red status - means I cannot apply there (or dont want to).\

Also each application has a status - sent, declined - no response, declined - with comment, in progress, accepted.

If company doesn't contact in 1 month - status is chenged to declined - no response. In progress

## Security

As this application will be placed on production with globasl access IP blocking will be provided.


## Tech stack
Application written in Go, with frontend in gohtml. Database simple noSQL for keeping information about company.


## Roadmap





7. stats + view dla pobranych
9. the protocol scrapper
