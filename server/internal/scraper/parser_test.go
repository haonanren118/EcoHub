package scraper

import (
	"testing"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTitle   string
		wantYear    string
		wantSeason  int
		wantEpisode int
		wantMovie   bool
	}{
		{
			name:        "标准电影带年份和4K分辨率",
			input:       "奥本海默.Oppenheimer.2023.2160p.HQ.WEB-DL.H265.mkv",
			wantTitle:   "奥本海默 Oppenheimer",
			wantYear:    "2023",
			wantSeason:  0,
			wantEpisode: 0,
			wantMovie:   true,
		},
		{
			name:        "标准美剧S01E01带年份",
			input:       "漫长的季节.The.Long.Season.2023.S01E01.4k.mkv",
			wantTitle:   "漫长的季节 The Long Season",
			wantYear:    "2023",
			wantSeason:  1,
			wantEpisode: 1,
			wantMovie:   false,
		},
		{
			name:        "国产剧EP格式",
			input:       "狂飙.2023.EP05.1080p.mkv",
			wantTitle:   "狂飙",
			wantYear:    "2023",
			wantSeason:  1,
			wantEpisode: 5,
			wantMovie:   false,
		},
		{
			name:        "第xx集中文格式",
			input:       "三体.第02集.2160p.mp4",
			wantTitle:   "三体",
			wantYear:    "",
			wantSeason:  1,
			wantEpisode: 2,
			wantMovie:   false,
		},
		{
			name:        "括号带年份的电影",
			input:       "星际穿越 (2014) [1080p BluRay x264].mp4",
			wantTitle:   "星际穿越",
			wantYear:    "2014",
			wantSeason:  0,
			wantEpisode: 0,
			wantMovie:   true,
		},
		{
			name:        "小雅strm电影格式",
			input:       "一个人的武林.2014.strm",
			wantTitle:   "一个人的武林",
			wantYear:    "2014",
			wantSeason:  0,
			wantEpisode: 0,
			wantMovie:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFilename(tt.input)
			if got.Title != tt.wantTitle {
				t.Errorf("ParseFilename() Title = %v, want %v", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Errorf("ParseFilename() Year = %v, want %v", got.Year, tt.wantYear)
			}
			if got.Season != tt.wantSeason {
				t.Errorf("ParseFilename() Season = %v, want %v", got.Season, tt.wantSeason)
			}
			if got.Episode != tt.wantEpisode {
				t.Errorf("ParseFilename() Episode = %v, want %v", got.Episode, tt.wantEpisode)
			}
			if got.IsMovie != tt.wantMovie {
				t.Errorf("ParseFilename() IsMovie = %v, want %v", got.IsMovie, tt.wantMovie)
			}
		})
	}
}
