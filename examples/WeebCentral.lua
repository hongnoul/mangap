--------------------------------
-- @name    WeebCentral
-- @url     https://weebcentral.com
-- @author  justinhong
-- @license MIT
--------------------------------

---@alias manga { name: string, url: string, author: string|nil, genres: string|nil, summary: string|nil }
---@alias chapter { name: string, url: string, volume: string|nil, manga_summary: string|nil, manga_author: string|nil, manga_genres: string|nil }
---@alias page { url: string, index: number }

----- IMPORTS -----
local http = require("http")
local html = require("html")
local strings = require("strings")
--- END IMPORTS ---

----- VARIABLES -----
local BASE = "https://weebcentral.com"
local client = http.client({ timeout = 30, user_agent = "Mozilla/5.0" })
--- END VARIABLES ---

----- HELPERS -----
local function get(url)
	local res, err = client:do_request(http.request("GET", url))
	if err then error(err) end
	if res.code ~= 200 then error("HTTP " .. res.code .. " for " .. url) end
	return html.parse(res.body)
end

local function urlencode(s)
	return (s:gsub("[^%w%-_%.~]", function(c)
		return string.format("%%%02X", string.byte(c))
	end))
end

local function trim(s)
	return strings.trim_space(s)
end
--- END HELPERS ---

----- MAIN -----

--- Searches for manga with given query.
-- @param query string Query to search for
-- @return manga[] Table of mangas
function SearchManga(query)
	local doc = get(BASE .. "/search/data?limit=32&offset=0&display_mode=Minimal+Display&text=" .. urlencode(query))
	local mangas = {}

	doc:find("article"):each(function(_, el)
		local link = el:find("a"):first()
		local url = link:attr("href")
		if url ~= "" then
			local genres = {}
			el:find("section span"):each(function(_, s)
				table.insert(genres, (trim(s:text()):gsub(",$", "")))
			end)
			table.insert(mangas, {
				name = trim(link:text()),
				url = url,
				genres = table.concat(genres, ", "),
			})
		end
	end)

	return mangas
end

--- Gets the list of all manga chapters.
-- @param mangaURL string URL of the manga
-- @return chapter[] Table of chapters
function MangaChapters(mangaURL)
	-- https://weebcentral.com/series/<ID>/<slug> -> /series/<ID>/full-chapter-list
	local id = mangaURL:match("/series/([^/]+)")
	local doc = get(BASE .. "/series/" .. id .. "/full-chapter-list")
	local chapters = {}

	doc:find("a[href*='/chapters/']"):each(function(_, a)
		local href = a:attr("href")
		if not href:match("^http") then href = BASE .. href end
		table.insert(chapters, {
			name = trim(a:find("span.grow > span"):first():text()),
			url = href,
		})
	end)

	-- Site lists newest first; mangaz expects oldest first.
	local ordered = {}
	for i = #chapters, 1, -1 do table.insert(ordered, chapters[i]) end
	return ordered
end

--- Gets the list of all pages of a chapter.
-- @param chapterURL string URL of the chapter
-- @return page[]
function ChapterPages(chapterURL)
	local doc = get(chapterURL .. "/images?is_prev=False&current_page=1&reading_style=long_strip")
	local pages = {}

	doc:find("section#chapter-images img"):each(function(i, img)
		local src = img:attr("src")
		if src ~= "" and not src:match("broken_image") then
			table.insert(pages, { url = src, index = i + 1 })
		end
	end)

	return pages
end

--- END MAIN ---

-- ex: ts=4 sw=4 et filetype=lua
