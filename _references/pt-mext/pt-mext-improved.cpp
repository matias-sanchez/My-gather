#include <iostream>
#include <fstream>
#include <string>
#include <stdlib.h>
#include <map>
using namespace std;

string get_var_name(string line);
int get_longest_var_name_length(std::fstream& _file);
long long int get_var_value(string line);
bool skip_line(string line);
string _pad(string s, int padLen, string padChar=" ");
string _trim(string s);

int main(int argc, char** argv){
  fstream newfile;
  string filename;
  if (argc < 2 ){
    filename="muestras-mysqladmin";

  } else {
    filename=argv[1];
  }


  newfile.open(filename,ios::in); 
  if (newfile.is_open()){   
    string line;
    string vname;
    long long vvalue;
    int col=0;
    int longest_vname_length;
    map <string, map <int, long long int> > result;
    map <string, long long int> previous;
    map <string, map <int, long long int> > stats;
    while(getline(newfile, line)){ 
      if(skip_line(line)){
        continue;
      }
      vname = get_var_name(line);
      vname = vname.substr(vname.find_first_not_of(" \t")).erase(vname.find_last_not_of(" \n\r\t")); 
      if (vname.compare("Variable_name") == 0) {     
        col++;
        continue;
      }
      vvalue = get_var_value(line);
      result[vname][col]=vvalue-previous[vname];
      previous[vname]=vvalue;
      if (col > 1) { // skip the first column which contains the initial tally
        stats[vname][0] += result[vname][col]; // total
        stats[vname][1] = (result[vname][col] < stats[vname][1] || col==2) ? result[vname][col] : stats[vname][1]; // min
        stats[vname][2] = result[vname][col] > stats[vname][2] ? result[vname][col] : stats[vname][2]; // max
        stats[vname][3] = stats[vname][0] / col; // avg
      }
    }

    // TODO: find out what's faster, if iterating once
    longest_vname_length = get_longest_var_name_length(newfile);
    // cout << "Longest var name: " << longest_vname_length << endl;

    map<string,map<int,long long int> >::iterator nextLine = result.begin();
    while (nextLine != result.end()) {
      cout << endl <<  _pad(nextLine->first, longest_vname_length, " ") << "\t"; // the variable name
      map <int, long long int>::iterator nextColumn = nextLine->second.begin();
      while (nextColumn != nextLine->second.end()) {\
        // TODO: pad
        cout << nextColumn->second << "\t"; // each column for that variable name
        nextColumn++;
      }
      cout << "\t | total: " << stats[nextLine->first][0] \
           << " min: " <<  stats[nextLine->first][1] \
           << " max: " << stats[nextLine->first][2] \
           << " avg: " <<  stats[nextLine->first][3];
      nextLine++;
    }
    cout << endl;
  }
}

// TODO: run just one iteration (i.e. one sample)
int get_longest_var_name_length(std::fstream& _file) {
  string line;
  string vname; 
  int max_len; 
  int loop;
    
  max_len=0;
  loop=0;  
  _file.clear();
  _file.seekg(0);

  if (_file.is_open()){   
    while(getline(_file, line)){ 
      if(skip_line(line)){
        continue;
      }
      vname = _trim(get_var_name(line));
      if (vname.compare("Variable_name") == 0) {
        if (loop == 0) {
          continue;
         } else{
          break;
         } 
        loop++;
      }
      if (vname.length() > max_len) {
        max_len = vname.length();
      }
    }
    return max_len;
  } else {
    return -1;
  }
}

string _pad(string s, int padLen, string padChar) {
  padLen = padLen - s.size();
  s.insert(s.end(), padLen, ' ');
  return s;
}
string _trim(string s) {
  return s.substr(s.find_first_not_of(" \t")).erase(s.find_last_not_of(" \n\r\t")); 
}

string  get_var_name(string line){
    return line.substr(1,line.find("|", 1)-1);
}

long long int get_var_value(string line){
    int cut_a = line.find("|", 1) + 1; 
    int cut_b = line.find("|", cut_a) - 1 - cut_a;
    char *ptr;
    return strtol(line.substr(cut_a, cut_b).c_str(), &ptr, 10);
}

bool skip_line (string line){
    return !(line.substr(0,1).compare("|") == 0);
}



/**
 * @brief Contents of mysqladmin
 * 

+----------------------+----------+
| Variable_name        | Value    |
+----------------------+----------+
| Aborted_clients      | 2589     |
| Aborted_connects     | 251      |
| Access_denied_errors | 121      |
| Acl_column_grants    | 0        |
| Acl_database_grants  | 6        |
| Acl_function_grants  | 0        |
+----------------------+----------+

+----------------------+----------+
| Variable_name        | Value    |
+----------------------+----------+
| Aborted_clients      | 2592     |
| Aborted_connects     | 254      |
| Access_denied_errors | 200      |
| Acl_column_grants    | 6        |
| Acl_database_grants  | 89       |
| Acl_function_grants  | 0        |
+----------------------+----------+

+----------------------+----------+
| Variable_name        | Value    |
+----------------------+----------+
| Aborted_clients      | 2599     |
| Aborted_connects     | 451      |
| Access_denied_errors | 500      |
| Acl_column_grants    | 22       |
| Acl_database_grants  | 96       |
| Acl_function_grants  | 0        |
+----------------------+----------+
*/ 

/**
 * @brief expected output
 * 
Aborted_clients         2589    3       7                | total: 10 min: 3 max: 7 avg: 3
Aborted_connects        251     3       197              | total: 200 min: 3 max: 197 avg: 66
Access_denied_errors    121     79      300              | total: 379 min: 79 max: 300 avg: 126
Acl_column_grants       0       6       16               | total: 22 min: 6 max: 16 avg: 7
Acl_database_grants     6       83      7                | total: 90 min: 7 max: 83 avg: 30
Acl_function_grants     0       0       0                | total: 0 min: 0 max: 0 avg: 0
 * 
 */
