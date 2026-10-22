#!/bin/sh
# Console API, served by DSM. Authentication uses the DSM login session.
exec /var/packages/StudioAlbum/target/bin/studioalbum cgi -data /var/packages/StudioAlbum/var
