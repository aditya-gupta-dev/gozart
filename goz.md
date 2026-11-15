Gozart currently is a simple tool that 
1) reads a link from links.txt 
2) downloads the video & extracts the audio from it 
3) applies a overlay(a pre-downloaded asset media) on the video 
4) merges the audio & overlay & loops it for x amount of hours specified in the config.json. 
5) uploads to youtube. 

for ex -> downloads video_id = 123. extracts audio. uses ffmpeg and merges it with asset. loops the video for 10 hours. uploads the video. 

That is the current status of the project. 

I want to transform it into something more massive. 

takes in a video idea & target audience as per country for ex. USA. And then researches about the topic in that country, searches for the most viral videos in that topic. gathers all the info. generates voice, and generates media with code. uploads that video to youtube. 

Here are the instructions for doing this project. 

1) First of all create a new git branch called etc. 
2) There should be AI integrated into this project. 
    And By AI I don't mean doing model API calling everywhere. 
    An AI Agent That does tool calling named [search-youtube-videos] which takes a query and returns youtube videos id & video metrics & their transcript based on it. 
    Then another LLM Analyzes the videos transcript & their performance then creates a new script based on it. 
    then another model call to generate the code for generate media in parallel to this. text-to-speech call to actually generate the voiceover. 
    a noise-compression model like deep-filter from facebook to reduce the noise in the text-to-speech voiceover. 

3) A Fine tuned text-to-image model to generate thumbnails. 
4) Another model to generate tags, description, title etc for Youtube SEO. 

First Tell where else can I add AI in this project. Or improve on the current workflow. 
