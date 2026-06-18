# Project Vision

## Pitch
WOK is a personal job-application tracker that helps a single job seeker (the author) stay organized during a job search by keeping track of every CV sent, the company applied to, and the current status of each application.

## Problem Statement
When applying to many companies over time with different tailored CV versions, it's easy to lose track of which CV went where, whether you've already applied to a company under a different name/alias, and what the current status of each application is. WOK solves this by giving a single source of truth for all applications.

## Target Users
A single user: the project's author, actively searching for a new job. No multi-tenant or team use case is planned.

## Key Features
- Track applications per company, including company aliases (to avoid duplicate applications to the same company under a different name)
- Record date of CV sent and CV version (versioned by date) to avoid CV overlap
- Attach screenshots of the job description and a link to the original posting
- Search applications by company name or alias
- Status tracking: sent, declined (no response), declined (with comment), in progress, accepted
- Automatic status transition: if a company hasn't responded within 1 month, status moves to "declined - no response"
- History of actions taken on an application,
- Red-flag/blacklist marking for companies the user cannot or does not want to apply to
- IP-based access blocking, since the app is exposed publicly but is for personal use only

## Success Criteria
- The author can reliably find out, for any company, whether they've already applied and with which CV
- No duplicate CV submissions to the same company under different aliases
- Application statuses stay accurate with minimal manual upkeep

## Differentiators
Purpose-built for one person's exact workflow (CV versioning + company aliasing + auto-expiry of stale applications) rather than a generic job-tracker — intentionally minimal in scope.
