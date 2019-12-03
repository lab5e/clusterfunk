package funk
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// nodeItem is a temporary struct used by the SerfNode and RaftNode types
// for their internal memberList methods.
// TODO: make separate structs for SerfNode/RaftNode?
type nodeItem struct {
	ID     string
	State  string
	Leader bool
}
